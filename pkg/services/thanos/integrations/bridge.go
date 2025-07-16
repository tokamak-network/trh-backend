package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// BridgeIntegration handles bridge installation and uninstallation
type BridgeIntegration struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
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
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	}
}

// NewBridgeIntegration creates a new bridge integration handler
func NewBridgeIntegration(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
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
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *BridgeIntegration {
	return &BridgeIntegration{
		stackRepo:       stackRepo,
		integrationRepo: integrationRepo,
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

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "bridge")
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
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-bridge-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.installTask(ctx, stack, sdkClient, logPath)
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

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "uninstall-bridge")
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
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-bridge-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.uninstallTask(ctx, stack, sdkClient, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (b *BridgeIntegration) installTask(ctx context.Context, stack *entities.StackEntity, sdkClient interface{}, logPath string) {
	bridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeBridge.String(),
		Status:  string(entities.DeploymentStatusInProgress),
		Config:  []byte("{}"),
		LogPath: logPath,
	}

	if err := b.integrationRepo.CreateIntegration(bridgeIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	thanosClient, ok := sdkClient.(*thanosStack.ThanosStack)
	if !ok {
		logger.Error("failed to type assert sdkClient", zap.String("plugin", enum.IntegrationTypeBridge.String()))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(bridgeIntegration.ID.String(), entities.DeploymentStatusFailed, "Invalid SDK client type"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(updateErr), zap.String("integrationId", bridgeIntegration.ID.String()))
		}
		return
	}

	bridgeUrl, err := thanos.InstallBridge(ctx, thanosClient)
	if err != nil {
		logger.Error("failed to install bridge", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(bridgeIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(updateErr), zap.String("integrationId", bridgeIntegration.ID.String()))
		}
		return
	}

	if bridgeUrl == "" {
		logger.Error("bridge URL is empty", zap.String("plugin", enum.IntegrationTypeBridge.String()))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(bridgeIntegration.ID.String(), entities.DeploymentStatusFailed, "Bridge URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(updateErr), zap.String("integrationId", bridgeIntegration.ID.String()))
		}
		return
	}

	logger.Debug("bridge successfully installed", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.String("url", bridgeUrl))

	config, err := json.Marshal(map[string]string{})
	if err != nil {
		logger.Error("failed to marshal bridge config", zap.Error(err))
		return
	}

	if err = b.integrationRepo.UpdateConfig(bridgeIntegration.ID.String(), json.RawMessage(config)); err != nil {
		logger.Error("failed to update bridge integration config", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	bridgeMetadata := map[string]string{"url": bridgeUrl}
	bytes, err := json.Marshal(bridgeMetadata)
	if err != nil {
		logger.Error("failed to marshal bridge metadata", zap.Error(err))
		return
	}

	if err = b.integrationRepo.UpdateMetadataAfterInstalled(bridgeIntegration.ID.String(), entities.IntegrationInfo(bytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	stack.Metadata.BridgeUrl = bridgeUrl
	if err = b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}
}

// uninstallTask handles the actual uninstallation process
func (b *BridgeIntegration) uninstallTask(ctx context.Context, stack *entities.StackEntity, sdkClient interface{}, stackId string) {
	integration, err := b.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeBridge.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	if integration == nil {
		logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeBridge.String()))
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	thanosClient, ok := sdkClient.(*thanosStack.ThanosStack)
	if !ok {
		logger.Error("failed to type assert sdkClient for uninstall", zap.String("plugin", enum.IntegrationTypeBridge.String()))
		return
	}

	if err = thanos.UninstallBridge(ctx, thanosClient); err != nil {
		logger.Error("failed to uninstall bridge", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
		return
	}

	stack.Metadata.BridgeUrl = ""
	if err = b.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}
}
