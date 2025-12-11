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

// UptimeServiceIntegration handles uptime service installation and uninstallation
type UptimeServiceIntegration struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
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
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
	}
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	}
}

// NewUptimeServiceIntegration creates a new uptime service integration handler
func NewUptimeServiceIntegration(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
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
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *UptimeServiceIntegration {
	return &UptimeServiceIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

// Install installs an uptime service for the given stack
func (u *UptimeServiceIntegration) Install(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := u.stackRepo.GetStackByID(stackId)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	// check if uptime service is already in non-terminated state
	integrations, err := u.integrationRepo.GetActiveIntegrations(stackId, enum.IntegrationTypeUptimeService.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active uptime service", zap.String("plugin", enum.IntegrationTypeUptimeService.String()))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active uptime service",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "system-pulse")

	uptimeServiceIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeUptimeService.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  []byte("{}"),
		LogPath: logPath,
	}

	if err := u.integrationRepo.CreateIntegration(uptimeServiceIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-system-pulse-%s", stackId)
	u.taskManager.AddTask(taskId, func(ctx context.Context) {
		u.installTask(ctx, stack, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// Uninstall uninstalls the uptime service for the given stack
func (u *UptimeServiceIntegration) Uninstall(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := u.stackRepo.GetStackByID(stackId)
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

	logPath := utils.GetLogPath(stack.ID, "uninstall-system-pulse")

	uptimeServiceIntegration, err := u.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeUptimeService.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	if uptimeServiceIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "System pulse integration not found",
			Data:    nil,
		}, nil
	}
	if err := u.integrationRepo.UpdateIntegrationStatus(uptimeServiceIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-system-pulse-%s", stackId)
	u.taskManager.AddTask(taskId, func(ctx context.Context) {
		u.uninstallTask(ctx, stack, stackId, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (u *UptimeServiceIntegration) installTask(ctx context.Context, stack *entities.StackEntity, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	uptimeServiceIntegration, err := u.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeUptimeService.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	if err := u.integrationRepo.UpdateIntegrationStatus(uptimeServiceIntegration.ID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	// Create deployment record for installing uptime service
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallUptimeServiceStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := u.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin installation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go u.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

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

	req := &dtos.InstallUptimeServiceRequest{}
	uptimeServiceUrl, err := thanos.InstallUptimeService(ctx, sdkClient, req)
	if err != nil {
		logger.Error("failed to install uptime service", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		if err := u.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed); err != nil {
			logger.Error("failed to update deployment status", zap.Error(err), zap.String("deploymentId", deployment.ID.String()))
		}
		err = u.integrationRepo.UpdateIntegrationStatusWithReason(uptimeServiceIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error())
		if err != nil {
			logger.Error("failed to update integration status", zap.Error(err), zap.String("integrationId", uptimeServiceIntegration.ID.String()))
		}
		return
	}

	if uptimeServiceUrl == "" {
		logger.Error("uptime service URL is empty", zap.String("plugin", enum.IntegrationTypeUptimeService.String()))
		if updateErr := u.integrationRepo.UpdateIntegrationStatusWithReason(uptimeServiceIntegration.ID.String(), entities.DeploymentStatusFailed, "Uptime service URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(updateErr), zap.String("integrationId", uptimeServiceIntegration.ID.String()))
		}
		_ = u.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	logger.Debug("uptime service successfully installed", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.String("url", uptimeServiceUrl))

	config, err := json.Marshal(map[string]string{})
	if err != nil {
		logger.Error("failed to marshal uptime service config", zap.Error(err))
		return
	}

	if err = u.integrationRepo.UpdateConfig(uptimeServiceIntegration.ID.String(), json.RawMessage(config)); err != nil {
		logger.Error("failed to update uptime service integration config", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	uptimeServiceMetadata := map[string]string{"url": uptimeServiceUrl}
	bytes, err := json.Marshal(uptimeServiceMetadata)
	if err != nil {
		logger.Error("failed to marshal uptime service metadata", zap.Error(err))
		return
	}

	if err = u.integrationRepo.UpdateMetadataAfterInstalled(uptimeServiceIntegration.ID.String(), entities.IntegrationInfo(bytes)); err != nil {
		logger.Error("failed to update integration metadata", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		if err := u.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed); err != nil {
			logger.Error("failed to update deployment status", zap.Error(err), zap.String("deploymentId", deployment.ID.String()))
		}
		return
	}

	// Update stack metadata if needed (similar to bridge)
	// Note: You may need to add UptimeServiceUrl field to StackMetadata if it doesn't exist
	// For now, we'll skip this if the field doesn't exist
	if stack.Metadata != nil {
		stack.Metadata.UptimeServiceUrl = uptimeServiceUrl
		if err = u.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stack.ID.String()), zap.Error(err))
			err = u.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
			if err != nil {
				logger.Error("failed to update deployment status", zap.Error(err), zap.String("deploymentId", deployment.ID.String()))
			}
			return
		}
	}

	err = u.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
	if err != nil {
		logger.Error("failed to update deployment status", zap.Error(err), zap.String("deploymentId", deployment.ID.String()))
		return
	}
}

// uninstallTask handles the actual uninstallation process
func (u *UptimeServiceIntegration) uninstallTask(ctx context.Context, stack *entities.StackEntity, stackId string, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	var integration *entities.IntegrationEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during uptime service uninstall", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = u.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			if integration != nil {
				_ = u.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
			}
		}
	}()

	integration, err := u.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeUptimeService.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	if integration == nil {
		logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeUptimeService.String()))
		return
	}

	if err = u.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	// Create deployment record for uninstalling uptime service
	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallUptimeServiceStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := u.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin uninstallation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go u.tailAndIngestLogs(ingestCtx, stack.ID, uninstallDeployment.ID, logPath)

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
	if err = thanos.UninstallUptimeService(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall uptime service", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		err = u.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		if err != nil {
			logger.Error("failed to update deployment status", zap.Error(err), zap.String("deploymentId", uninstallDeployment.ID.String()))
		}
		err = u.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, err.Error())
		if err != nil {
			logger.Error("failed to update integration status", zap.Error(err), zap.String("integrationId", integration.ID.String()))
		}
		return
	}

	if err = u.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	// Update stack metadata if needed
	if stack.Metadata != nil {
		stack.Metadata.UptimeServiceUrl = ""
		if err = u.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
			return
		}
	}

	err = u.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
	if err != nil {
		logger.Error("failed to update deployment status", zap.Error(err), zap.String("deploymentId", uninstallDeployment.ID.String()))
		return
	}
}

// tailAndIngestLogs tails a log file and ingests each line into the database
func (u *UptimeServiceIntegration) tailAndIngestLogs(
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
					if dbErr := u.logRepo.CreateLog(l); dbErr != nil {
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
