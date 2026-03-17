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
		StopTask(id string)
		IsTaskRunning(id string) bool
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
		StopTask(id string)
		IsTaskRunning(id string) bool
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

	logPath := utils.GetLogPath(stack.ID, "system-pulse")

	if len(integrations) > 0 {
		// If all active integrations are pre-created Pending entries (from preset auto-creation),
		// reuse the first one instead of blocking the install request.
		allPending := true
		for _, intg := range integrations {
			if intg.Status != string(entities.DeploymentStatusPending) {
				allPending = false
				break
			}
		}
		if !allPending {
			logger.Error("There is already an active uptime service", zap.String("plugin", enum.IntegrationTypeUptimeService.String()))
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "There is already an active uptime service",
				Data:    nil,
			}, nil
		}
		// Reuse pre-created Pending integration
		existingIntegration := integrations[0]
		taskId := fmt.Sprintf("install-system-pulse-%s", stackId)
		u.taskManager.AddTask(taskId, func(ctx context.Context) {
			u.installTask(ctx, existingIntegration.ID, stack, logPath)
		})
		return &entities.Response{
			Status:  http.StatusOK,
			Message: "Successfully",
			Data:    nil,
		}, nil
	}

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
		u.installTask(ctx, uptimeServiceIntegration.ID, stack, logPath)
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
		u.uninstallTask(ctx, uptimeServiceIntegration.ID, stack, stackId, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (u *UptimeServiceIntegration) installTask(ctx context.Context, newIntegrationID uuid.UUID, stack *entities.StackEntity, logPath string) {
	// Create context with 30min timeout to prevent infinite running installations
	taskCtx, taskCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer taskCancel()

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	if err := u.integrationRepo.UpdateIntegrationStatus(newIntegrationID.String(), entities.DeploymentStatusInProgress); err != nil {
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
		taskCtx,
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
	uptimeServiceUrl, err := thanos.InstallUptimeService(taskCtx, sdkClient, req)
	if err != nil {
		logger.Error("failed to install uptime service", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))

		if updateErr := u.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
		}

		deploymentStatus := entities.DeploymentRunStatusFailed
		if errors.Is(err, context.Canceled) {
			deploymentStatus = entities.DeploymentRunStatusStopped
		}
		_ = u.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), deploymentStatus)
		return
	}

	if uptimeServiceUrl == "" {
		logger.Error("uptime service URL is empty", zap.String("plugin", enum.IntegrationTypeUptimeService.String()))
		if updateErr := u.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, "Uptime service URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
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

	if err = u.integrationRepo.UpdateConfig(newIntegrationID.String(), json.RawMessage(config)); err != nil {
		logger.Error("failed to update uptime service integration config", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Error(err))
		return
	}

	uptimeServiceMetadata := map[string]string{"url": uptimeServiceUrl}
	bytes, err := json.Marshal(uptimeServiceMetadata)
	if err != nil {
		logger.Error("failed to marshal uptime service metadata", zap.Error(err))
		return
	}

	if err = u.integrationRepo.UpdateMetadataAfterInstalled(newIntegrationID.String(), entities.IntegrationInfo(bytes)); err != nil {
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
func (u *UptimeServiceIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, stackId string, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during uptime service uninstall", zap.String("plugin", enum.IntegrationTypeUptimeService.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = u.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			_ = u.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
		}
	}()

	if err := u.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
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
		err = u.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error())
		if err != nil {
			logger.Error("failed to update integration status", zap.Error(err), zap.String("integrationId", integrationID.String()))
		}
		return
	}

	if err = u.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated); err != nil {
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

// Cancel sets the cancellation flag , it detects and handle cleanup
func (u *UptimeServiceIntegration) Cancel(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	// 1 Fetch integration
	integration, err := u.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		logger.Error("failed to get integration", zap.Error(err), zap.String("integrationId", integrationId.String()))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if integration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Integration not found",
			Data:    nil,
		}, nil
	}

	// 2 Validate status: can only cancel if inprogress or pending
	if integration.Status != string(entities.DeploymentStatusInProgress) && integration.Status != string(entities.DeploymentStatusPending) {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Can only cancel installations that are in progress or pending",
			Data:    nil,
		}, nil
	}

	if err = u.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelling,
		"Stopping installation process. This may take a few minutes to safely clean up AWS resources.",
	); err != nil {
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Failed to update status"}, err
	}

	u.taskManager.AddTask(fmt.Sprintf("cancel-system-pulse-%s", stackId.String()), func(ctx context.Context) {

		// Stop the running task to cancel the context immediately
		taskId := fmt.Sprintf("install-system-pulse-%s", stackId.String())
		u.taskManager.StopTask(taskId)

		stack, err := u.stackRepo.GetStackByID(stackId.String())
		if err != nil {
			logger.Error("failed to get stack", zap.Error(err), zap.String("stackId", stackId.String()))
			return
		}
		stackConfig := dtos.DeployThanosRequest{}
		if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
			logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
			return
		}

		sdkClient, err := thanos.NewThanosSDKClient(
			ctx,
			utils.GetLogPath(stack.ID, "cancel-system-pulse"),
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
			logger.Error("failed to uninstall uptime service", zap.Error(err))
			_ = u.integrationRepo.UpdateIntegrationStatusWithReason(
				integration.ID.String(),
				entities.DeploymentStatusFailed,
				err.Error(),
			)
			return
		}

		logger.Info("Cancellation requested successfully", zap.String("integrationId", integrationId.String()))
		_ = u.integrationRepo.UpdateIntegrationStatusWithReason(
			integration.ID.String(),
			entities.DeploymentStatusCancelled,
			"Installation cancelled successfully. All AWS resources have been cleaned up.",
		)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Cancellation in progress. Installation will be stopped and AWS resources will be cleaned up. This may take 2-3 minutes for safe cleanup.",
		Data:    nil,
	}, nil
}

func (u *UptimeServiceIntegration) Retry(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return retryIntegrationCommon(ctx, stackId, integrationId, u.integrationRepo, u.stackRepo,
		func(stack *entities.StackEntity, integration *entities.IntegrationEntity) error {
			logPath := utils.GetLogPath(stack.ID, "install-system-pulse")

			// no config needed for uptime, just retry the installation
			taskId := fmt.Sprintf("install-%s-%s", integration.Type, stackId.String())
			u.taskManager.AddTask(taskId, func(ctx context.Context) {
				u.installTask(ctx, integration.ID, stack, logPath)
			})

			return nil
		})
}
