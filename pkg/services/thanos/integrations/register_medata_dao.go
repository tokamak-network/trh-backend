package integrations

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// RegisterMetadataDAOIntegration handles register metadata dao operations
type RegisterMetadataDAOIntegration struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	}
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	}
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
	}
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	}
}

// NewRegisterMetadataDAOIntegration creates a new register metadata dao integration handler
func NewRegisterMetadataDAOIntegration(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	},
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	},
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *RegisterMetadataDAOIntegration {
	return &RegisterMetadataDAOIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

// Register registers a metadata dao for the given stack
func (r *RegisterMetadataDAOIntegration) Register(ctx context.Context, stackId uuid.UUID, req dtos.RegisterMetadataDAORequest) (*entities.Response, error) {
	stack, err := r.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack by id", zap.Error(err))
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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack has not been deployed yet",
			Data:    nil,
		}, nil
	}

	// check if register metadata dao is already in InProgress state
	integrations, err := r.integrationRepo.GetActiveIntegrations(stackId.String(), enum.IntegrationTypeRegisterMetadataDAO.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 && integrations[0].Status == string(entities.DeploymentStatusInProgress) {
		logger.Error("There is already an active register metadata dao in progress", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active register metadata dao in progress",
			Data:    nil,
		}, nil
	}

	stackConfig := dtos.DeployThanosRequest{}
	err = json.Unmarshal(stack.Config, &stackConfig)
	if err != nil {
		logger.Error("failed to unmarshal stack config", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	registerMetadataDaoLogPath := utils.GetLogPath(stackId, "register-metadata-dao")

	integrationConfig, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal integration config", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	registerMetadataDaoIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeRegisterMetadataDAO.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  integrationConfig,
		LogPath: registerMetadataDaoLogPath,
	}

	if err := r.integrationRepo.CreateIntegration(registerMetadataDaoIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("register-metadata-dao-%s", stackId.String())
	r.taskManager.AddTask(taskId, func(ctx context.Context) {
		r.registerTask(ctx, stack, req, registerMetadataDaoLogPath, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Metadata DAO registered successfully",
		Data:    nil,
	}, nil
}

// registerTask handles the actual registration process
func (r *RegisterMetadataDAOIntegration) registerTask(ctx context.Context, stack *entities.StackEntity, req dtos.RegisterMetadataDAORequest, logPath string, stackId uuid.UUID) {
	integrationConfig, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal integration config", zap.Error(err))
		return
	}

	registerMetadataDaoIntegration, err := r.integrationRepo.GetInstalledIntegration(stackId.String(), enum.IntegrationTypeRegisterMetadataDAO.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return
	}
	if err := r.integrationRepo.UpdateIntegrationStatus(registerMetadataDaoIntegration.ID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return
	}

	// Create deployment record for register metadata dao
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stackId,
		Step:    constants.RegisterMetadataDAOStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  integrationConfig,
	}
	if err := r.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return
	}

	stackConfig := dtos.DeployThanosRequest{}
	err = json.Unmarshal(stack.Config, &stackConfig)
	if err != nil {
		logger.Error("failed to unmarshal stack config", zap.Error(err))
		return
	}
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
		return
	}

	// Start log ingestion for this register metadata dao operation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go r.tailAndIngestLogs(ingestCtx, stackId, deployment.ID, logPath)

	registerMedataDaoResult, err := thanos.RegisterMetadataDAO(ctx, sdkClient, &req)
	if err != nil {
		logger.Error("failed to register metadata dao", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err), zap.String("stackId", stackId.String()))
		if updateErr := r.integrationRepo.UpdateIntegrationStatusWithReason(registerMetadataDaoIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(updateErr), zap.String("integrationId", registerMetadataDaoIntegration.ID.String()))
		}
		_ = r.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if err = r.integrationRepo.UpdateIntegrationStatus(registerMetadataDaoIntegration.ID.String(), entities.DeploymentStatusCompleted); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err), zap.String("integrationId", registerMetadataDaoIntegration.ID.String()))
	}

	bytes, err := json.Marshal(registerMedataDaoResult)
	if err != nil {
		logger.Error("failed to marshal register metadata dao info", zap.Error(err))
		return
	}

	if err = r.integrationRepo.UpdateMetadataAfterInstalled(registerMetadataDaoIntegration.ID.String(), bytes); err != nil {
		logger.Error("failed to update register metadata dao integration metadata", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return
	}

	logger.Info("Register metadata dao successfully", zap.String("stackId", stackId.String()))

	_ = r.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// tailAndIngestLogs tails a log file and ingests each line into the database
func (r *RegisterMetadataDAOIntegration) tailAndIngestLogs(
	ctx context.Context,
	stackID uuid.UUID,
	deploymentID uuid.UUID,
	logPath string,
) {
	// Wait for file to appear
	for {
		if _, err := os.Stat(logPath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		logger.Error("failed to open log file", zap.String("path", logPath), zap.Error(err))
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				msg := strings.TrimRight(line, "\r\n")
				if msg != "" {
					l := &entities.LogEntity{
						StackID:      &stackID,
						DeploymentID: &deploymentID,
						Message:      msg,
					}
					if dbErr := r.logRepo.CreateLog(l); dbErr != nil {
						logger.Error("failed to insert log", zap.Error(dbErr))
					}
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(300 * time.Millisecond)
					continue
				}
				logger.Error("error reading log file", zap.Error(err))
				return
			}
		}
	}
}

func (r *RegisterMetadataDAOIntegration) Get(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	integrations, err := r.integrationRepo.GetActiveIntegrations(stackId.String(), enum.IntegrationTypeRegisterMetadataDAO.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeRegisterMetadataDAO.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) == 0 {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Register metadata dao not found",
			Data:    nil,
		}, nil
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Register metadata dao found",
		Data:    map[string]interface{}{"config": integrations[0].Config, "info": integrations[0].Info},
	}, nil
}
