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

// MonitoringIntegration handles monitoring installation and uninstallation
type MonitoringIntegration struct {
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

// NewMonitoringIntegration creates a new monitoring integration handler
func NewMonitoringIntegration(
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
) *MonitoringIntegration {
	return &MonitoringIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
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

	logPath := utils.GetLogPath(stack.ID, "monitoring")

	// save config for retry: monitoring has a lot of settings we need to preserve
	requestConfig, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal request config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		// If all active integrations are pre-created AwaitingConfig entries (from preset), transition
		// the first one to installation using the now-provided configuration.
		allAwaitingConfig := true
		for _, intg := range integrations {
			if intg.Status != string(entities.DeploymentStatusAwaitingConfig) {
				allAwaitingConfig = false
				break
			}
		}
		if !allAwaitingConfig {
			logger.Error("There is already an active monitoring", zap.String("plugin", "monitoring"))
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "There is already an active monitoring",
				Data:    nil,
			}, nil
		}
		// Transition AwaitingConfig integration: save config and launch install task
		existingIntegration := integrations[0]
		if err := m.integrationRepo.UpdateConfig(existingIntegration.ID.String(), json.RawMessage(requestConfig)); err != nil {
			logger.Error("failed to update integration config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return &entities.Response{
				Status:  http.StatusInternalServerError,
				Message: "Internal server error",
				Data:    nil,
			}, err
		}
		taskId := fmt.Sprintf("install-monitoring-%s", stackId)
		m.taskManager.AddTask(taskId, func(ctx context.Context) {
			m.installTask(ctx, existingIntegration.ID, stack, req, logPath, stackId.String())
		})
		return &entities.Response{
			Status:  http.StatusOK,
			Message: "Successfully",
			Data:    nil,
		}, nil
	}

	monitoringIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeMonitoring.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  requestConfig,
		LogPath: logPath,
	}

	if err := m.integrationRepo.CreateIntegration(monitoringIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-monitoring-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.installTask(ctx, monitoringIntegration.ID, stack, req, logPath, stackId.String())
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

	logPath := utils.GetLogPath(stack.ID, "uninstall-monitoring")

	monitoringIntegration, _ := m.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeMonitoring.String())
	if monitoringIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Monitoring integration not found",
			Data:    nil,
		}, nil
	}

	if err := m.integrationRepo.UpdateIntegrationStatus(monitoringIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-monitoring-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.uninstallTask(ctx, monitoringIntegration.ID, stack, stackId.String(), logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (m *MonitoringIntegration) installTask(ctx context.Context, newIntegrationID uuid.UUID, stack *entities.StackEntity, req dtos.InstallMonitoringRequest, logPath string, stackId string) {
	// Create context with 30min timeout to prevent infinite running installations
	taskCtx, taskCancel := context.WithTimeout(ctx, 30*time.Minute)
	defer taskCancel()

	configBytes, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal monitoring config", zap.Error(err))
		return
	}

	if err := m.integrationRepo.UpdateIntegrationStatus(newIntegrationID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	// Create deployment record for installing monitoring
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallMonitoringStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  configBytes,
	}
	if err := m.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	sdkClient, err := thanos.NewSDKClientForStack(taskCtx, logPath, stack, &stackConfig)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	monitoringConfig, err := thanos.GetMonitoringConfig(taskCtx, sdkClient, &req)
	if err != nil {
		logger.Error("failed to get monitoring config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	// Start log ingestion for this plugin installation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go m.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

	monitoringInfo, err := thanos.InstallMonitoring(taskCtx, sdkClient, monitoringConfig)
	if err != nil {
		logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))

		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
		}
		deploymentStatus := entities.DeploymentRunStatusFailed
		if errors.Is(err, context.Canceled) {
			deploymentStatus = entities.DeploymentRunStatusStopped
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), deploymentStatus)
		return
	}

	if monitoringInfo == nil {
		logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, "Failed to install monitoring"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if monitoringInfo.GrafanaURL == "" {
		logger.Error("monitoring URL is empty", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(newIntegrationID.String(), entities.DeploymentStatusFailed, "Monitoring URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", newIntegrationID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	logger.Debug("monitoring successfully installed", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.String("url", monitoringInfo.GrafanaURL))

	config, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal monitoring config", zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if err = m.integrationRepo.UpdateConfig(newIntegrationID.String(), json.RawMessage(config)); err != nil {
		logger.Error("failed to update monitoring integration config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	// Convert AlertManager config to consistent camelCase format for storage
	alertManagerForStorage := m.convertAlertManagerToCamelCase(req.AlertManager)

	monitoringMetadata := map[string]interface{}{
		"url":           monitoringInfo.GrafanaURL,
		"username":      monitoringInfo.Username,
		"password":      monitoringInfo.Password,
		"alert_manager": alertManagerForStorage,
	}
	bytes, err := json.Marshal(monitoringMetadata)
	if err != nil {
		logger.Error("failed to marshal monitoring metadata", zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if err = m.integrationRepo.UpdateMetadataAfterInstalled(newIntegrationID.String(), entities.IntegrationInfo(bytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	stack.Metadata.GrafanaUrl = monitoringInfo.GrafanaURL
	if err = m.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// uninstallTask handles the actual uninstallation process
func (m *MonitoringIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, stackId string, logPath string) {
	var uninstallDeployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during monitoring uninstall", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = m.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			_ = m.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
		}
	}()
	var err error
	if err = m.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	// Create deployment record for uninstalling monitoring
	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallMonitoringStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := m.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin uninstallation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go m.tailAndIngestLogs(ingestCtx, stack.ID, uninstallDeployment.ID, logPath)

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	sdkClient, err := thanos.NewSDKClientForStack(ctx, logPath, stack, &stackConfig)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	if err = thanos.UninstallMonitoring(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = m.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = m.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	stack.Metadata.GrafanaUrl = ""
	if err = m.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}

	_ = m.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// DisableEmailAlert disables email alert configuration for the given stack
func (m *MonitoringIntegration) DisableEmailAlert(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	// Check if monitoring is installed
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId.String(), enum.IntegrationTypeMonitoring.String())
	if err != nil {
		logger.Error("failed to get monitoring integration", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if monitoringIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Monitoring integration not found",
			Data:    nil,
		}, nil
	}

	// Update the database synchronously first so frontend gets immediate feedback
	if err := m.updateMonitoringConfigInDB(ctx, stackId.String(), "email", false); err != nil {
		logger.Error("failed to update email configuration in database", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to update email configuration",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "disable-email-alert")

	// Start background task to update AlertManager configuration via SDK
	taskId := fmt.Sprintf("disable-email-alert-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.disableEmailAlertTask(ctx, stack, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Email alert disabled successfully",
		Data:    nil,
	}, nil
}

// DisableTelegramAlert disables telegram alert configuration for the given stack
func (m *MonitoringIntegration) DisableTelegramAlert(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	// Check if monitoring is installed
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId.String(), enum.IntegrationTypeMonitoring.String())
	if err != nil {
		logger.Error("failed to get monitoring integration", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if monitoringIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Monitoring integration not found",
			Data:    nil,
		}, nil
	}

	// Update the database synchronously first so frontend gets immediate feedback
	if err := m.updateMonitoringConfigInDB(ctx, stackId.String(), "telegram", false); err != nil {
		logger.Error("failed to update telegram configuration in database", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to update telegram configuration",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "disable-telegram-alert")

	// Start background task to update AlertManager configuration via SDK
	taskId := fmt.Sprintf("disable-telegram-alert-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.disableTelegramAlertTask(ctx, stack, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Telegram alert disabled successfully",
		Data:    nil,
	}, nil
}

// disableEmailAlertTask handles the actual email alert disabling process
func (m *MonitoringIntegration) disableEmailAlertTask(ctx context.Context, stack *entities.StackEntity, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	sdkClient, err := thanos.NewSDKClientForStack(ctx, logPath, stack, &stackConfig)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	if err = thanos.RemoveEmailConfig(ctx, sdkClient); err != nil {
		logger.Error("failed to remove email configuration", zap.Error(err))
		return
	}

	// Update the database again to ensure consistency after SDK operation
	// (Note: DB was already updated synchronously before this background task)
	if err := m.updateMonitoringConfigInDB(ctx, stack.ID.String(), "email", false); err != nil {
		logger.Error("failed to update email configuration in database after SDK operation", zap.String("stackId", stack.ID.String()), zap.Error(err))
		// Don't return here as the SDK operation was successful
	}

	logger.Info("Email alert configuration removed successfully", zap.String("stackId", stack.ID.String()))
}

// disableTelegramAlertTask handles the actual telegram alert disabling process
func (m *MonitoringIntegration) disableTelegramAlertTask(ctx context.Context, stack *entities.StackEntity, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	sdkClient, err := thanos.NewSDKClientForStack(ctx, logPath, stack, &stackConfig)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	if err = thanos.RemoveTelegramConfig(ctx, sdkClient); err != nil {
		logger.Error("failed to remove telegram configuration", zap.Error(err))
		return
	}

	// Update the database again to ensure consistency after SDK operation
	// (Note: DB was already updated synchronously before this background task)
	if err := m.updateMonitoringConfigInDB(ctx, stack.ID.String(), "telegram", false); err != nil {
		logger.Error("failed to update telegram configuration in database after SDK operation", zap.String("stackId", stack.ID.String()), zap.Error(err))
		// Don't return here as the SDK operation was successful
	}

	logger.Info("Telegram alert configuration removed successfully", zap.String("stackId", stack.ID.String()))
}

// UpdateEmailAlert updates email alert configuration for the given stack
func (m *MonitoringIntegration) UpdateEmailAlert(ctx context.Context, stackId uuid.UUID, request dtos.UpdateEmailConfigRequest) (*entities.Response, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		}, nil
	}

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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	// Check if monitoring is installed
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId.String(), enum.IntegrationTypeMonitoring.String())
	if err != nil {
		logger.Error("failed to get monitoring integration", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if monitoringIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Monitoring integration not found",
			Data:    nil,
		}, nil
	}

	// Update the database synchronously first so frontend gets immediate feedback
	if err := m.updateMonitoringConfigInDBWithEmailConfig(ctx, stackId.String(), request); err != nil {
		logger.Error("failed to update email configuration in database", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to update email configuration",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "update-email-alert")

	// Start background task to update AlertManager configuration via SDK
	taskId := fmt.Sprintf("update-email-alert-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.updateEmailAlertTask(ctx, stack, logPath, request)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Email alert configuration updated successfully",
		Data:    nil,
	}, nil
}

// UpdateTelegramAlert updates telegram alert configuration for the given stack
func (m *MonitoringIntegration) UpdateTelegramAlert(ctx context.Context, stackId uuid.UUID, request dtos.UpdateTelegramConfigRequest) (*entities.Response, error) {
	// Validate request
	if err := request.Validate(); err != nil {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		}, nil
	}

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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	// Check if monitoring is installed
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId.String(), enum.IntegrationTypeMonitoring.String())
	if err != nil {
		logger.Error("failed to get monitoring integration", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if monitoringIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Monitoring integration not found",
			Data:    nil,
		}, nil
	}

	// Update the database synchronously first so frontend gets immediate feedback
	if err := m.updateMonitoringConfigInDBWithTelegramConfig(ctx, stackId.String(), request); err != nil {
		logger.Error("failed to update telegram configuration in database", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to update telegram configuration",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "update-telegram-alert")

	// Start background task to update AlertManager configuration via SDK
	taskId := fmt.Sprintf("update-telegram-alert-%s", stackId)
	m.taskManager.AddTask(taskId, func(ctx context.Context) {
		m.updateTelegramAlertTask(ctx, stack, logPath, request)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Telegram alert configuration updated successfully",
		Data:    nil,
	}, nil
}

// updateEmailAlertTask handles the actual email alert updating process
func (m *MonitoringIntegration) updateEmailAlertTask(ctx context.Context, stack *entities.StackEntity, logPath string, request dtos.UpdateEmailConfigRequest) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	sdkClient, err := thanos.NewSDKClientForStack(ctx, logPath, stack, &stackConfig)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	if err = thanos.UpdateEmailConfig(ctx, sdkClient, &request); err != nil {
		logger.Error("failed to update email configuration", zap.Error(err))
		return
	}

	// Update the database again to ensure consistency after SDK operation
	// (Note: DB was already updated synchronously before this background task)
	if err := m.updateMonitoringConfigInDBWithEmailConfig(ctx, stack.ID.String(), request); err != nil {
		logger.Error("failed to update email configuration in database after SDK operation", zap.String("stackId", stack.ID.String()), zap.Error(err))
		// Don't return here as the SDK operation was successful
	}

	logger.Info("Email alert configuration updated successfully", zap.String("stackId", stack.ID.String()))
}

// updateTelegramAlertTask handles the actual telegram alert updating process
func (m *MonitoringIntegration) updateTelegramAlertTask(ctx context.Context, stack *entities.StackEntity, logPath string, request dtos.UpdateTelegramConfigRequest) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	sdkClient, err := thanos.NewSDKClientForStack(ctx, logPath, stack, &stackConfig)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	if err = thanos.UpdateTelegramConfig(ctx, sdkClient, &request); err != nil {
		logger.Error("failed to update telegram configuration", zap.Error(err))
		return
	}

	// Update the database again to ensure consistency after SDK operation
	// (Note: DB was already updated synchronously before this background task)
	if err := m.updateMonitoringConfigInDBWithTelegramConfig(ctx, stack.ID.String(), request); err != nil {
		logger.Error("failed to update telegram configuration in database after SDK operation", zap.String("stackId", stack.ID.String()), zap.Error(err))
		// Don't return here as the SDK operation was successful
	}

	logger.Info("Telegram alert configuration updated successfully", zap.String("stackId", stack.ID.String()))
}

// updateMonitoringConfigInDBWithEmailConfig updates the monitoring integration configuration with new email config
func (m *MonitoringIntegration) updateMonitoringConfigInDBWithEmailConfig(ctx context.Context, stackId string, request dtos.UpdateEmailConfigRequest) error {
	// Get the monitoring integration
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeMonitoring.String())
	if err != nil {
		return fmt.Errorf("failed to get monitoring integration: %w", err)
	}

	if monitoringIntegration == nil {
		return fmt.Errorf("monitoring integration not found")
	}

	// Parse the existing configuration
	var currentConfig dtos.InstallMonitoringRequest
	if len(monitoringIntegration.Config) > 0 {
		if err := json.Unmarshal(monitoringIntegration.Config, &currentConfig); err != nil {
			logger.Warn("failed to parse existing monitoring config, creating new one", zap.Error(err))
			// Initialize with default values if parsing fails
			currentConfig = dtos.InstallMonitoringRequest{
				AlertManager: dtos.AlertManagerConfig{
					Email:    dtos.EmailConfig{Enabled: false},
					Telegram: dtos.TelegramConfig{Enabled: false},
				},
			}
		}
	} else {
		// Initialize with default values if no config exists
		currentConfig = dtos.InstallMonitoringRequest{
			AlertManager: dtos.AlertManagerConfig{
				Email:    dtos.EmailConfig{Enabled: false},
				Telegram: dtos.TelegramConfig{Enabled: false},
			},
		}
	}

	// Update the email configuration
	currentConfig.AlertManager.Email.Enabled = true
	currentConfig.AlertManager.Email.SmtpSmarthost = request.SmtpSmarthost
	currentConfig.AlertManager.Email.SmtpFrom = request.SmtpFrom
	currentConfig.AlertManager.Email.SmtpAuthPassword = request.SmtpAuthPassword
	currentConfig.AlertManager.Email.AlertReceivers = request.AlertReceivers

	// Marshal the updated configuration
	updatedConfigBytes, err := json.Marshal(currentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Update the integration configuration in the database
	if err := m.integrationRepo.UpdateConfig(monitoringIntegration.ID.String(), json.RawMessage(updatedConfigBytes)); err != nil {
		return fmt.Errorf("failed to update integration config: %w", err)
	}

	// Also update the integration metadata (info field) to reflect the current alert configuration
	if err := m.updateIntegrationMetadata(ctx, monitoringIntegration, currentConfig); err != nil {
		logger.Error("failed to update integration metadata", zap.Error(err))
		// Don't return error here as the main config update was successful
	}

	logger.Info("Successfully updated email configuration in database", zap.String("stackId", stackId))

	return nil
}

// updateMonitoringConfigInDBWithTelegramConfig updates the monitoring integration configuration with new telegram config
func (m *MonitoringIntegration) updateMonitoringConfigInDBWithTelegramConfig(ctx context.Context, stackId string, request dtos.UpdateTelegramConfigRequest) error {
	// Get the monitoring integration
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeMonitoring.String())
	if err != nil {
		return fmt.Errorf("failed to get monitoring integration: %w", err)
	}

	if monitoringIntegration == nil {
		return fmt.Errorf("monitoring integration not found")
	}

	// Parse the existing configuration
	var currentConfig dtos.InstallMonitoringRequest
	if len(monitoringIntegration.Config) > 0 {
		if err := json.Unmarshal(monitoringIntegration.Config, &currentConfig); err != nil {
			logger.Warn("failed to parse existing monitoring config, creating new one", zap.Error(err))
			// Initialize with default values if parsing fails
			currentConfig = dtos.InstallMonitoringRequest{
				AlertManager: dtos.AlertManagerConfig{
					Email:    dtos.EmailConfig{Enabled: false},
					Telegram: dtos.TelegramConfig{Enabled: false},
				},
			}
		}
	} else {
		// Initialize with default values if no config exists
		currentConfig = dtos.InstallMonitoringRequest{
			AlertManager: dtos.AlertManagerConfig{
				Email:    dtos.EmailConfig{Enabled: false},
				Telegram: dtos.TelegramConfig{Enabled: false},
			},
		}
	}

	// Update the telegram configuration
	currentConfig.AlertManager.Telegram.Enabled = true
	currentConfig.AlertManager.Telegram.ApiToken = request.ApiToken
	currentConfig.AlertManager.Telegram.CriticalReceivers = request.CriticalReceivers

	// Marshal the updated configuration
	updatedConfigBytes, err := json.Marshal(currentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Update the integration configuration in the database
	if err := m.integrationRepo.UpdateConfig(monitoringIntegration.ID.String(), json.RawMessage(updatedConfigBytes)); err != nil {
		return fmt.Errorf("failed to update integration config: %w", err)
	}

	// Also update the integration metadata (info field) to reflect the current alert configuration
	if err := m.updateIntegrationMetadata(ctx, monitoringIntegration, currentConfig); err != nil {
		logger.Error("failed to update integration metadata", zap.Error(err))
		// Don't return error here as the main config update was successful
	}

	logger.Info("Successfully updated telegram configuration in database", zap.String("stackId", stackId))

	return nil
}

// updateMonitoringConfigInDB updates the monitoring integration configuration in the database
func (m *MonitoringIntegration) updateMonitoringConfigInDB(ctx context.Context, stackId, alertType string, enabled bool) error {
	// Get the monitoring integration
	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeMonitoring.String())
	if err != nil {
		return fmt.Errorf("failed to get monitoring integration: %w", err)
	}

	if monitoringIntegration == nil {
		return fmt.Errorf("monitoring integration not found")
	}

	// Parse the existing configuration
	var currentConfig dtos.InstallMonitoringRequest
	if len(monitoringIntegration.Config) > 0 {
		if err := json.Unmarshal(monitoringIntegration.Config, &currentConfig); err != nil {
			logger.Warn("failed to parse existing monitoring config, creating new one", zap.Error(err))
			// Initialize with default values if parsing fails
			currentConfig = dtos.InstallMonitoringRequest{
				AlertManager: dtos.AlertManagerConfig{
					Email:    dtos.EmailConfig{Enabled: false},
					Telegram: dtos.TelegramConfig{Enabled: false},
				},
			}
		}
	} else {
		// Initialize with default values if no config exists
		currentConfig = dtos.InstallMonitoringRequest{
			AlertManager: dtos.AlertManagerConfig{
				Email:    dtos.EmailConfig{Enabled: false},
				Telegram: dtos.TelegramConfig{Enabled: false},
			},
		}
	}

	// Update the specific alert type configuration
	switch alertType {
	case "email":
		currentConfig.AlertManager.Email.Enabled = enabled
		if !enabled {
			// Clear email configuration when disabled
			currentConfig.AlertManager.Email.SmtpSmarthost = ""
			currentConfig.AlertManager.Email.SmtpFrom = ""
			currentConfig.AlertManager.Email.SmtpAuthPassword = ""
			currentConfig.AlertManager.Email.AlertReceivers = []string{}
		}
	case "telegram":
		currentConfig.AlertManager.Telegram.Enabled = enabled
		if !enabled {
			// Clear telegram configuration when disabled
			currentConfig.AlertManager.Telegram.ApiToken = ""
			currentConfig.AlertManager.Telegram.CriticalReceivers = []dtos.TelegramReceiver{}
		}
	default:
		return fmt.Errorf("unsupported alert type: %s", alertType)
	}

	// Marshal the updated configuration
	updatedConfigBytes, err := json.Marshal(currentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Update the integration configuration in the database
	if err := m.integrationRepo.UpdateConfig(monitoringIntegration.ID.String(), json.RawMessage(updatedConfigBytes)); err != nil {
		return fmt.Errorf("failed to update integration config: %w", err)
	}

	// Also update the integration metadata (info field) to reflect the current alert configuration
	if err := m.updateIntegrationMetadata(ctx, monitoringIntegration, currentConfig); err != nil {
		logger.Error("failed to update integration metadata", zap.Error(err))
		// Don't return error here as the main config update was successful
	}

	logger.Info("Successfully updated monitoring configuration in database",
		zap.String("stackId", stackId),
		zap.String("alertType", alertType),
		zap.Bool("enabled", enabled))

	return nil
}

// updateIntegrationMetadata updates the integration metadata (info field) with current alert configuration
func (m *MonitoringIntegration) updateIntegrationMetadata(ctx context.Context, integration *entities.IntegrationEntity, config dtos.InstallMonitoringRequest) error {
	// Parse existing metadata to preserve other fields like URL, username, password
	var existingMetadata map[string]interface{}
	if len(integration.Info) > 0 {
		if err := json.Unmarshal(integration.Info, &existingMetadata); err != nil {
			logger.Warn("failed to parse existing integration metadata, creating new one", zap.Error(err))
			existingMetadata = make(map[string]interface{})
		}
	} else {
		existingMetadata = make(map[string]interface{})
	}

	// Update the alert_manager configuration in metadata with consistent camelCase format
	existingMetadata["alert_manager"] = m.convertAlertManagerToCamelCase(config.AlertManager)

	// Marshal the updated metadata
	updatedMetadataBytes, err := json.Marshal(existingMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal updated metadata: %w", err)
	}

	// Update the integration metadata in the database
	if err := m.integrationRepo.UpdateMetadataAfterInstalled(integration.ID.String(), entities.IntegrationInfo(updatedMetadataBytes)); err != nil {
		return fmt.Errorf("failed to update integration metadata: %w", err)
	}

	logger.Debug("Successfully updated integration metadata",
		zap.String("integrationId", integration.ID.String()),
		zap.Bool("emailEnabled", config.AlertManager.Email.Enabled),
		zap.Bool("telegramEnabled", config.AlertManager.Telegram.Enabled))

	return nil
}

// convertAlertManagerToCamelCase converts AlertManager config to consistent camelCase format for storage
func (m *MonitoringIntegration) convertAlertManagerToCamelCase(alertManager dtos.AlertManagerConfig) map[string]interface{} {
	return map[string]interface{}{
		"email": map[string]interface{}{
			"enabled":          alertManager.Email.Enabled,
			"smtpSmarthost":    alertManager.Email.SmtpSmarthost,
			"smtpFrom":         alertManager.Email.SmtpFrom,
			"smtpAuthPassword": alertManager.Email.SmtpAuthPassword,
			"alertReceivers":   alertManager.Email.AlertReceivers,
		},
		"telegram": map[string]interface{}{
			"enabled":           alertManager.Telegram.Enabled,
			"apiToken":          alertManager.Telegram.ApiToken,
			"criticalReceivers": alertManager.Telegram.CriticalReceivers,
		},
	}
}

// tailAndIngestLogs tails a log file and ingests each line into the database
func (m *MonitoringIntegration) tailAndIngestLogs(
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
					if dbErr := m.logRepo.CreateLog(l); dbErr != nil {
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

// Cancel sets the cancellation flag and handles cancel
func (m *MonitoringIntegration) Cancel(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	// 1 Fetch integration
	integration, err := m.integrationRepo.GetIntegrationById(integrationId.String())
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
	if err = m.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelling,
		"Stopping installation process. This may take a few minutes to safely clean up AWS resources.",
	); err != nil {
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Failed to update status"}, err
	}

	m.taskManager.AddTask(fmt.Sprintf("cancel-monitoring-%s", stackId.String()), func(ctx context.Context) {
		// then stop the running task to cancel the context immediately
		taskId := fmt.Sprintf("install-monitoring-%s", stackId.String())
		m.taskManager.StopTask(taskId)

		stack, err := m.stackRepo.GetStackByID(stackId.String())
		if err != nil {
			logger.Error("failed to get stack", zap.Error(err), zap.String("stackId", stackId.String()))
			return
		}
		stackConfig := dtos.DeployThanosRequest{}
		if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
			logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
			return
		}
		sdkClient, err := thanos.NewSDKClientForStack(
			ctx,
			utils.GetLogPath(stack.ID, "cancel-monitoring"),
			stack,
			&stackConfig,
		)
		if err := thanos.UninstallMonitoring(ctx, sdkClient); err != nil {
			logger.Error("failed to uninstall monitoring", zap.Error(err))

			_ = m.integrationRepo.UpdateIntegrationStatusWithReason(
				integration.ID.String(),
				entities.DeploymentStatusFailed,
				err.Error(),
			)
			return
		}
		logger.Info("Cancellation requested successfully", zap.String("integrationId", integrationId.String()))
		_ = m.integrationRepo.UpdateIntegrationStatusWithReason(
			integration.ID.String(),
			entities.DeploymentStatusCancelled,
			"Installation cancelled successfully. All AWS resources have been cleaned up.",
		)

		logger.Info("Cancellation completed successfully", zap.String("integrationId", integrationId.String()))
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Cancellation in progress. Installation will be stopped and AWS resources will be cleaned up. This may take 3-5 minutes for safe cleanup.",
		Data:    nil,
	}, nil
}

func (m *MonitoringIntegration) Retry(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return retryIntegrationCommon(ctx, stackId, integrationId, m.integrationRepo, m.stackRepo,
		func(stack *entities.StackEntity, integration *entities.IntegrationEntity) error {
			logPath := utils.GetLogPath(stack.ID, "install-monitoring")

			// grab the saved config (grafana password, alert settings, etc)
			var request dtos.InstallMonitoringRequest
			if integration.Config == nil || len(integration.Config) == 0 || string(integration.Config) == "{}" {
				logger.Error("installation config is missing or empty", zap.String("integrationId", integrationId.String()))
				return &BadRequestError{message: "Cannot retry installation: original configuration not found. Please uninstall and reinstall instead."}
			}

			if err := json.Unmarshal(integration.Config, &request); err != nil {
				logger.Error("failed to unmarshal config", zap.Error(err), zap.String("integrationId", integrationId.String()))
				return fmt.Errorf("Failed to retrieve installation config")
			}

			// start installation again
			taskId := fmt.Sprintf("install-%s-%s", integration.Type, stackId.String())
			m.taskManager.AddTask(taskId, func(ctx context.Context) {
				m.installTask(ctx, integration.ID, stack, request, logPath, stackId.String())
			})

			return nil
		})
}
