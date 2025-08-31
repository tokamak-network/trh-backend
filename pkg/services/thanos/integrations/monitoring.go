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

// NewMonitoringIntegration creates a new monitoring integration handler
func NewMonitoringIntegration(
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

	if len(integrations) > 0 {
		logger.Error("There is already an active monitoring", zap.String("plugin", "monitoring"))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active monitoring",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "monitoring")

	monitoringIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeMonitoring.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  []byte("{}"),
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
		m.installTask(ctx, stack, req, logPath, stackId.String())
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
		m.uninstallTask(ctx, stack, stackId.String(), logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (m *MonitoringIntegration) installTask(ctx context.Context, stack *entities.StackEntity, req dtos.InstallMonitoringRequest, logPath string, stackId string) {
	configBytes, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal monitoring config", zap.Error(err))
		return
	}

	monitoringIntegration, err := m.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeMonitoring.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		return
	}

	if err := m.integrationRepo.UpdateIntegrationStatus(monitoringIntegration.ID.String(), entities.DeploymentStatusInProgress); err != nil {
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

	monitoringConfig, err := thanos.GetMonitoringConfig(ctx, sdkClient, &req)
	if err != nil {
		logger.Error("failed to get monitoring config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	// Start log ingestion for this plugin installation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go m.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

	monitoringInfo, err := thanos.InstallMonitoring(ctx, sdkClient, monitoringConfig)
	if err != nil {
		logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if monitoringInfo == nil {
		logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, "Failed to install monitoring"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if monitoringInfo.GrafanaURL == "" {
		logger.Error("monitoring URL is empty", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
		if updateErr := m.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, "Monitoring URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(updateErr), zap.String("integrationId", monitoringIntegration.ID.String()))
		}
		_ = m.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
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
func (m *MonitoringIntegration) uninstallTask(ctx context.Context, stack *entities.StackEntity, stackId string, logPath string) {
	var uninstallDeployment *entities.DeploymentEntity
	var integration *entities.IntegrationEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during monitoring uninstall", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = m.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			if integration != nil {
				_ = m.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
			}
		}
	}()
	var err error
	integration, err = m.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeMonitoring.String())
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

	if err = thanos.UninstallMonitoring(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
		_ = m.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = m.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = m.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated); err != nil {
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
