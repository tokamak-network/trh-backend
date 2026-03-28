package integrations

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// DRBIntegration handles DRB uninstallation
type DRBIntegration struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
	}
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
		GetDeploymentByStepAndStatus(stackID string, step string, status entities.DeploymentStatus) (*entities.DeploymentEntity, error)
	}
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
	}
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64)))
		SetTaskResult(id string, result any)
		StopTask(id string)
		IsTaskRunning(id string) bool
	}
}

func NewDRBIntegration(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
	},
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
		GetDeploymentByStepAndStatus(stackID string, step string, status entities.DeploymentStatus) (*entities.DeploymentEntity, error)
	},
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64)))
		SetTaskResult(id string, result any)
		StopTask(id string)
		IsTaskRunning(id string) bool
	},
) *DRBIntegration {
	return &DRBIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

func (d *DRBIntegration) Uninstall(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := d.stackRepo.GetStackByID(stackId)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	logPath := fmt.Sprintf("/tmp/uninstall-drb-%s.log", stackId)

	drbIntegration, err := d.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeDRB.String())
	if err != nil {
		logger.Error("failed to get DRB integration", zap.String("stackId", stackId), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	if drbIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "DRB integration not found",
			Data:    nil,
		}, nil
	}

	if err := d.integrationRepo.UpdateIntegrationStatus(drbIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update DRB integration status", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-drb-%s", stackId)
	d.taskManager.AddTask(taskId, func(ctx context.Context) {
		d.uninstallTask(ctx, drbIntegration.ID, stack, stackId, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (d *DRBIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, stackId string, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during DRB uninstall", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = d.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
		}
	}()

	if err := d.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update DRB integration status to Terminating", zap.Error(err))
		return
	}

	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallDRBStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := d.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall DRB deployment record", zap.Error(err))
		return
	}

	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go d.tailAndIngestLogs(ingestCtx, stack.ID, uninstallDeployment.ID, logPath)

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		_ = d.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = thanos.UninstallDRB(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall DRB", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		_ = d.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = d.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update DRB integration status to Terminated", zap.Error(err))
		return
	}

	_ = d.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

func (d *DRBIntegration) tailAndIngestLogs(ctx context.Context, stackID uuid.UUID, deploymentID uuid.UUID, logPath string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var file *os.File
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if file == nil {
				file, err = os.Open(logPath)
				if err != nil {
					continue
				}
				defer file.Close()
			}
			reader := bufio.NewReader(file)
			for {
				line, err := reader.ReadString('\n')
				if line != "" {
					_ = d.logRepo.CreateLog(&entities.LogEntity{
						ID:           uuid.New(),
						StackID:      &stackID,
						DeploymentID: &deploymentID,
						Message:      line,
					})
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
			}
		}
	}
}
