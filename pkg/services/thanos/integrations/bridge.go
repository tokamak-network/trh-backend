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

// BridgeIntegration handles bridge installation and uninstallation
type BridgeIntegration struct {
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
		GetUninstallableIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		DeleteIntegration(id string) error
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

// NewBridgeIntegration creates a new bridge integration handler
func NewBridgeIntegration(
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
		GetUninstallableIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		DeleteIntegration(id string) error
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
) *BridgeIntegration {
	return &BridgeIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

// Install installs a bridge for the given stack
func (b *BridgeIntegration) Install(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId)
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

	// check if bridge is already in non-terminated state
	integrations, err := b.integrationRepo.GetActiveIntegrations(stackId, "bridge")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "bridge"), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active bridge", zap.String("plugin", "bridge"))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active bridge",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "bridge")

	bridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeBridge.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  []byte("{}"),
		LogPath: logPath,
	}

	if err := b.integrationRepo.CreateIntegration(bridgeIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-bridge-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.installTask(ctx, bridgeIntegration.ID, stack, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// Uninstall uninstalls the bridge for the given stack
func (b *BridgeIntegration) Uninstall(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId)
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

	logPath := utils.GetLogPath(stack.ID, "uninstall-bridge")

	bridgeIntegration, _ := b.integrationRepo.GetUninstallableIntegration(stack.ID.String(), enum.IntegrationTypeBridge.String())
	if bridgeIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Bridge integration not found",
			Data:    nil,
		}, nil
	}
	if err := b.integrationRepo.UpdateIntegrationStatus(bridgeIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-bridge-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.uninstallTask(ctx, bridgeIntegration.ID, stack, stackId, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (b *BridgeIntegration) installTask(ctx context.Context, newIntegrationID uuid.UUID, stack *entities.StackEntity, logPath string) {
	// creates context with 30min timeout so to prevent infinite running installations
	taskCtx, taskCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer taskCancel()

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(newIntegrationID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	// Create deployment record for installing bridge
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallBridgeStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin installation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go b.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

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
	bridgeUrl, err := thanos.InstallBridge(taskCtx, sdkClient)
	if err != nil {
		logger.Error("failed to install bridge", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))

		// Auto cleanup: attempt to remove any partially created resources
		logger.Info("attempting automatic cleanup of failed bridge installation", zap.String("integrationId", newIntegrationID.String()))
		if cleanupErr := thanos.UninstallBridge(taskCtx, sdkClient); cleanupErr != nil {
			logger.Warn("automatic cleanup failed", zap.String("integrationId", newIntegrationID.String()), zap.Error(cleanupErr))
			// Cleanup failed - mark as Failed so user can manually uninstall
			reason := fmt.Sprintf("installation failed: %s; cleanup failed: %s", err.Error(), cleanupErr.Error())
			if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, reason); updateErr != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
			}
		} else {
			logger.Info("automatic cleanup successful, removing integration record", zap.String("integrationId", newIntegrationID.String()))
			// Cleanup succeeded - delete the integration record
			if deleteErr := b.integrationRepo.DeleteIntegration(newIntegrationID.String()); deleteErr != nil {
				logger.Error("failed to delete integration after cleanup", zap.String("integrationId", newIntegrationID.String()), zap.Error(deleteErr))
			}
		}

		deploymentStatus := entities.DeploymentRunStatusFailed
		if utils.IsContextCanceled(err) {
			deploymentStatus = entities.DeploymentRunStatusStopped
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), deploymentStatus)
		return
	}

	if bridgeUrl == "" {
		logger.Error("bridge URL is empty", zap.String("plugin", enum.IntegrationTypeBridge.String()))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, "Bridge URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	logger.Debug("bridge successfully installed", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.String("url", bridgeUrl))

	config, err := json.Marshal(map[string]string{})
	if err != nil {
		logger.Error("failed to marshal bridge config", zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if err = b.integrationRepo.UpdateConfig(newIntegrationID.String(), json.RawMessage(config)); err != nil {
		logger.Error("failed to update bridge integration config", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	bridgeMetadata := map[string]string{"url": bridgeUrl}
	bytes, err := json.Marshal(bridgeMetadata)
	if err != nil {
		logger.Error("failed to marshal bridge metadata", zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if err = b.integrationRepo.UpdateMetadataAfterInstalled(newIntegrationID.String(), entities.IntegrationInfo(bytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	stack.Metadata.BridgeUrl = bridgeUrl
	if err = b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stack.ID.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// uninstallTask handles the actual uninstallation process
func (b *BridgeIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, stackId string, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during bridge uninstall", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
		}
	}()

	if err := b.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	// Create deployment record for uninstalling bridge
	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallBridgeStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin uninstallation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go b.tailAndIngestLogs(ingestCtx, stack.ID, uninstallDeployment.ID, logPath)

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
	if err = thanos.UninstallBridge(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall bridge", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	stack.Metadata.BridgeUrl = ""
	if err = b.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// tailAndIngestLogs tails a log file and ingests each line into the database
func (b *BridgeIntegration) tailAndIngestLogs(
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
					if dbErr := b.logRepo.CreateLog(l); dbErr != nil {
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

// Cancel sets the cancellation flag the task will be detect and do cleanup
func (b *BridgeIntegration) Cancel(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	// 1 Fetch integration
	integration, err := b.integrationRepo.GetIntegrationById(integrationId.String())
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

	// Set the integration status to cancelling
	if err = b.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelling,
		"Stopping installation process. This may take several minutes to safely clean up AWS resources (ECS tasks, networking).",
	); err != nil {
		logger.Error("failed to request cancellation", zap.Error(err), zap.String("integrationId", integrationId.String()))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to request cancellation",
			Data:    nil,
		}, err
	}

	b.taskManager.AddTask(fmt.Sprintf("cancel-bridge-%s", stackId.String()), func(ctx context.Context) {
		// then stop the running task to cancel the context immediately
		taskId := fmt.Sprintf("install-bridge-%s", stackId.String())
		b.taskManager.StopTask(taskId)

		stack, err := b.stackRepo.GetStackByID(stackId.String())
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
			utils.GetLogPath(stack.ID, "cancel-bridge"),
			string(stack.Network),
			stack.DeploymentPath,
			stackConfig.RegisterCandidate,
			stackConfig.AwsAccessKey,
			stackConfig.AwsSecretAccessKey,
			stackConfig.AwsRegion,
		)

		if err = thanos.UninstallBridge(ctx, sdkClient); err != nil {
			logger.Error("failed to uninstall bridge", zap.Error(err))

			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
				integration.ID.String(),
				entities.DeploymentStatusFailed,
				err.Error(),
			)
			return
		}

		logger.Info("Cancellation requested successfully", zap.String("integrationId", integrationId.String()))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			integration.ID.String(),
			entities.DeploymentStatusCancelled,
			"Installation cancelled successfully. All AWS resources have been cleaned up.",
		)

	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Cancellation in progress. Installation will be stopped and AWS resources will be cleaned up. This may take 3-5 minutes for safe cleanup.",
		Data:    nil,
	}, nil
}

// SyncBlockExplorerURL updates the bridge pod's L2 block explorer URL using the
// URL already stored in stack metadata. Used for existing deployments where the
// block-explorer came up after the bridge was installed.
func (b *BridgeIntegration) SyncBlockExplorerURL(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId)
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

	if stack.Metadata == nil || stack.Metadata.ExplorerUrl == "" {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Block explorer URL is not available for this stack yet",
			Data:    nil,
		}, nil
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		"",
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to create SDK client",
			Data:    nil,
		}, err
	}

	if err := thanos.UpdateBridgeBlockExplorer(ctx, sdkClient, stack.Metadata.ExplorerUrl); err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to update bridge block explorer URL: %s", err.Error()),
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Bridge block explorer URL updated successfully",
		Data:    map[string]string{"explorerUrl": stack.Metadata.ExplorerUrl},
	}, nil
}

func (b *BridgeIntegration) Retry(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return retryIntegrationCommon(ctx, stackId, integrationId, b.integrationRepo, b.stackRepo,
		func(stack *entities.StackEntity, integration *entities.IntegrationEntity) error {
			logPath := utils.GetLogPath(stack.ID, "install-bridge")

			// bridge doesnt need any config, just kick it off again
			taskId := fmt.Sprintf("install-%s-%s", integration.Type, stackId.String())
			b.taskManager.AddTask(taskId, func(ctx context.Context) {
				b.installTask(ctx, integration.ID, stack, logPath)
			})

			return nil
		})
}
