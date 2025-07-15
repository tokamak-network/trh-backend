package thanos

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
	"go.uber.org/zap"
)

func (s *ThanosStackDeploymentService) InstallBlockExplorer(ctx context.Context, stackId string, request dtos.InstallBlockExplorerRequest) (*entities.Response, error) {
	if err := request.Validate(); err != nil {
		logger.Error("invalid block explorer request", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid block explorer request",
			Data:    nil,
		}, err
	}

	stack, err := s.stackRepo.GetStackByID(stackId)
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

	// check if block explorer is already in non-terminated state
	integrations, err := s.integrationRepo.GetActiveIntegrations(stackId, "block-explorer")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "block-explorer"), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active block explorer", zap.String("plugin", "block-explorer"))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active block explorer",
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

	var (
		blockExplorerUrl string
	)

	logPath := utils.GetLogPath(stack.ID, "block-explorer")
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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-block-explorer-%s", stackId)
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		confifgBytes, err := json.Marshal(request)
		if err != nil {
			logger.Error("failed to marshal block explorer config", zap.Error(err))
			return
		}
		blockExplorerIntegration := &entities.IntegrationEntity{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Type:    enum.IntegrationTypeBlockExplorer.String(),
			Status:  string(entities.DeploymentStatusInProgress),
			Config:  confifgBytes,
			LogPath: logPath,
		}
		err = s.integrationRepo.CreateIntegration(blockExplorerIntegration)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}

		blockExplorerUrl, err = thanos.InstallBlockExplorer(ctx, sdkClient, &request)
		if err != nil {
			logger.Error("failed to install block explorer", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(blockExplorerIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error())
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err), zap.String("integrationId", blockExplorerIntegration.ID.String()))
				return
			}
			return
		}

		if blockExplorerUrl == "" {
			logger.Error("block explorer URL is empty", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(blockExplorerIntegration.ID.String(), entities.DeploymentStatusFailed, "Block explorer URL is empty")
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err), zap.String("integrationId", blockExplorerIntegration.ID.String()))
				return
			}
			return
		}

		logger.Debug("block explorer successfully installed", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.String("url", blockExplorerUrl))
		// create integration
		config, err := json.Marshal(request)
		if err != nil {
			logger.Error("failed to marshal block explorer config", zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateConfig(
			blockExplorerIntegration.ID.String(),
			json.RawMessage(config),
		)
		if err != nil {
			logger.Error("failed to update block explorer integration config", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}

		blockExplorerMedata := map[string]string{
			"url": blockExplorerUrl,
		}
		bytes, err := json.Marshal(blockExplorerMedata)
		if err != nil {
			logger.Error("failed to marshal block explorer metadata", zap.Error(err))
			return
		}
		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			blockExplorerIntegration.ID.String(),
			entities.IntegrationInfo(bytes),
		)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}
		stack.Metadata.BlockExplorerUrl = blockExplorerUrl

		err = s.stackRepo.UpdateMetadata(
			stackId,
			stack.Metadata,
		)
		if err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) UninstallBlockExplorer(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId)
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

	logPath := utils.GetLogPath(stack.ID, "uninstall-block-explorer")
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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-block-explorer-%s", stackId)
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		integration, err := s.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeBlockExplorer.String())
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()))
			return
		}
		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}
		err = thanos.UninstallBlockExplorer(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install block-explorer", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
			return
		}
		stack.Metadata.BlockExplorerUrl = ""

		err = s.stackRepo.UpdateMetadata(
			stackId,
			stack.Metadata,
		)
		if err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) InstallBridge(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId)
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
	integrations, err := s.integrationRepo.GetActiveIntegrations(stackId, "bridge")
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

	var (
		bridgeUrl string
	)

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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-bridge-%s", stackId)
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		bridgeIntegration := &entities.IntegrationEntity{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Type:    enum.IntegrationTypeBridge.String(),
			Status:  string(entities.DeploymentStatusInProgress),
			Config:  []byte("{}"),
			LogPath: logPath,
		}
		err = s.integrationRepo.CreateIntegration(bridgeIntegration)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}

		bridgeUrl, err = thanos.InstallBridge(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install bridge", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(bridgeIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error())
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err), zap.String("integrationId", bridgeIntegration.ID.String()))
				return
			}
			return
		}

		if bridgeUrl == "" {
			logger.Error("bridge URL is empty", zap.String("plugin", enum.IntegrationTypeBridge.String()))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(bridgeIntegration.ID.String(), entities.DeploymentStatusFailed, "Bridge URL is empty")
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err), zap.String("integrationId", bridgeIntegration.ID.String()))
				return
			}
			return
		}

		logger.Debug("bridge successfully installed", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.String("url", bridgeUrl))
		// create integration
		config, err := json.Marshal(map[string]string{})
		if err != nil {
			logger.Error("failed to marshal bridge config", zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateConfig(
			bridgeIntegration.ID.String(),
			json.RawMessage(config),
		)
		if err != nil {
			logger.Error("failed to update bridge integration config", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}

		bridgeMetadata := map[string]string{
			"url": bridgeUrl,
		}
		bytes, err := json.Marshal(bridgeMetadata)
		if err != nil {
			logger.Error("failed to marshal bridge metadata", zap.Error(err))
			return
		}
		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			bridgeIntegration.ID.String(),
			entities.IntegrationInfo(bytes),
		)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}
		stack.Metadata.BridgeUrl = bridgeUrl

		err = s.stackRepo.UpdateMetadata(
			stackId,
			stack.Metadata,
		)
		if err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) UninstallBridge(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId)
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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-bridge-%s", stackId)
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		integration, err := s.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeBridge.String())
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeBridge.String()))
			return
		}
		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}
		err = thanos.UninstallBridge(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to uninstall bridge", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBridge.String()), zap.Error(err))
			return
		}
		stack.Metadata.BridgeUrl = ""

		err = s.stackRepo.UpdateMetadata(
			stackId,
			stack.Metadata,
		)
		if err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) InstallMonitoring(
	ctx context.Context,
	stackId uuid.UUID,
	req dtos.InstallMonitoringRequest,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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
	integrations, err := s.integrationRepo.GetActiveIntegrations(stackId.String(), "monitoring")
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

	var (
		monitoringUrl string
	)

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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-monitoring-%s", stackId)
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
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
		err = s.integrationRepo.CreateIntegration(monitoringIntegration)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}

		monitoringConfig, err := thanos.GetMonitoringConfig(ctx, sdkClient, req.GrafanaPassword)
		if err != nil {
			logger.Error("failed to get monitoring config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error())
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err), zap.String("integrationId", monitoringIntegration.ID.String()))
				return
			}
			return
		}

		monitoringUrl, err = thanos.InstallMonitoring(ctx, sdkClient, monitoringConfig)
		if err != nil {
			logger.Error("failed to install monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error())
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err), zap.String("integrationId", monitoringIntegration.ID.String()))
				return
			}
			return
		}

		if monitoringUrl == "" {
			logger.Error("monitoring URL is empty", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(monitoringIntegration.ID.String(), entities.DeploymentStatusFailed, "Monitoring URL is empty")
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err), zap.String("integrationId", monitoringIntegration.ID.String()))
				return
			}
			return
		}

		logger.Debug("monitoring successfully installed", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.String("url", monitoringUrl))
		// create integration
		config, err := json.Marshal(req)
		if err != nil {
			logger.Error("failed to marshal monitoring config", zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateConfig(
			monitoringIntegration.ID.String(),
			json.RawMessage(config),
		)
		if err != nil {
			logger.Error("failed to update monitoring integration config", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}

		monitoringMetadata := map[string]string{
			"url": monitoringUrl,
		}
		bytes, err := json.Marshal(monitoringMetadata)
		if err != nil {
			logger.Error("failed to marshal monitoring metadata", zap.Error(err))
			return
		}
		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			monitoringIntegration.ID.String(),
			entities.IntegrationInfo(bytes),
		)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}
		stack.Metadata.MonitoringUrl = monitoringUrl

		err = s.stackRepo.UpdateMetadata(
			stackId.String(),
			stack.Metadata,
		)
		if err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) UninstallMonitoring(
	ctx context.Context,
	stackId uuid.UUID,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-monitoring-%s", stackId)
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		integration, err := s.integrationRepo.GetInstalledIntegration(stackId.String(), enum.IntegrationTypeMonitoring.String())
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeMonitoring.String()))
			return
		}
		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}
		err = thanos.UninstallMonitoring(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to uninstall monitoring", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeMonitoring.String()), zap.Error(err))
			return
		}
		stack.Metadata.MonitoringUrl = ""

		err = s.stackRepo.UpdateMetadata(
			stackId.String(),
			stack.Metadata,
		)
		if err != nil {
			logger.Error("failed to update stack metadata", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}
