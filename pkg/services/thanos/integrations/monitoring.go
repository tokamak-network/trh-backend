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

// MonitoringIntegration handles monitoring installation and uninstallation
type MonitoringIntegration struct {
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

// NewMonitoringIntegration creates a new monitoring integration handler
func NewMonitoringIntegration(
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
) *MonitoringIntegration {
	return &MonitoringIntegration{
		stackRepo:       stackRepo,
		integrationRepo: integrationRepo,
		taskManager:     taskManager,
	}
}

// Install installs monitoring for the given stack
func (m *MonitoringIntegration) Install(ctx context.Context, stackId uuid.UUID, req dtos.InstallMonitoringRequest) (*entities.Response, error) {
	stack, err := m.stackRepo.GetStackByID(stackId.String())
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

	// check if monitoring is already in non-terminated state
	integrations, err := m.integrationRepo.GetActiveIntegrations(stackId.String(), "monitoring")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "monitoring"), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active monitoring", zap.String("plugin", "monitoring"))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active monitoring",
			Data:    nil,
		}, nil
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "monitoring")
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

	taskId := fmt.Sprintf("install-monitoring-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.installTask(ctx, stack, sdkClient, req, logPath, stackId.String())
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// Uninstall uninstalls the monitoring for the given stack
func (m *MonitoringIntegration) Uninstall(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := m.stackRepo.GetStackByID(stackId.String())
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
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "uninstall-monitoring")
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

	taskId := fmt.Sprintf("uninstall-monitoring-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.uninstallTask(ctx, stack, sdkClient, stackId.String())
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (m *MonitoringIntegration) installTask(ctx context.Context, stack *entities.StackEntity, sdkClient interface{}, req dtos.InstallMonitoringRequest, logPath string, stackId string) {
	configBytes, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal monitoring config", zap.Error(err))
		return
	}

	monitoringIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeMonitoring.String(),
		Status:  string(entities.DeploymentStatusInProgress),
		Config:  configBytes,
		LogPath: logPath,
	}

	if err = m.integrationRepo.CreateIntegration(monitoringIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	thanosClient, ok := sdkClient.(*thanosStack.ThanosStack)
	if !ok {
		logger.Error("failed to type assert sdkClient", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, "Invalid SDK client type"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		return
	}

	monitoringConfig, err := thanos.GetMonitoringConfig(ctx, thanosClient, req.GrafanaPassword, req.AlertManager)
	if err != nil {
		logger.Error("failed to get monitoring config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		return
	}

	monitoringInfo, err := thanos.InstallMonitoring(ctx, thanosClient, monitoringConfig)
	if err != nil {
		logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		return
	}

	if monitoringInfo == nil {
		logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, "Failed to install monitoring"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		return
	}

	if monitoringInfo.GrafanaURL == "" {
		logger.Error("monitoring URL is empty", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, "Monitoring URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		return
	}

	logger.Debug("monitoring successfully installed", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.String("url", monitoringInfo.GrafanaURL))

	config, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal monitoring config", zap.Error(err))
		return
	}

	if err = m.integrationRepo.UpdateConfig(monitoringIntegration.ID.String(), json.RawMessage(config)); err != nil {
		logger.Error("failed to update monitoring integration config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	monitoringMetadata := map[string]interface{}{
		"url":           monitoringInfo.GrafanaURL,
		"username":      monitoringInfo.Username,
		"password":      monitoringInfo.Password,
		"alert_manager": monitoringConfig.AlertManager,
	}
	bytes, err := json.Marshal(monitoringMetadata)
	if err != nil {
		logger.Error("failed to marshal monitoring metadata", zap.Error(err))
		return
	}

	if err = m.integrationRepo.UpdateMetadataAfterInstalled(monitoringIntegration.ID.String(), entities.IntegrationInfo(bytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	stack.Metadata.MonitoringUrl = monitoringInfo.GrafanaURL
	if err = m.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}
}

// uninstallTask handles the actual uninstallation process
func (m *MonitoringIntegration) uninstallTask(ctx context.Context, stack *entities.StackEntity, sdkClient interface{}, stackId string) {
	integration, err := m.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeMonitoring.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	if integration == nil {
		logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		return
	}

	if err = m.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	thanosClient, ok := sdkClient.(*thanosStack.ThanosStack)
	if !ok {
		logger.Error("failed to type assert sdkClient for uninstall", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		return
	}

	if err = thanos.UninstallMonitoring(ctx, thanosClient); err != nil {
		logger.Error("failed to uninstall monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	if err = m.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	stack.Metadata.MonitoringUrl = ""
	if err = m.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}
}
