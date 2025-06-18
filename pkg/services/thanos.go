package services

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
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	"go.uber.org/zap"
)

type DeploymentRepository interface {
	GetDeploymentsByStackID(stackId string) ([]*entities.DeploymentEntity, error)
	UpdateDeploymentStatus(deploymentId string, status entities.DeploymentStatus) error
	GetDeploymentByID(deploymentId string) (*entities.DeploymentEntity, error)
	GetDeploymentStatus(deploymentId string) (entities.DeploymentStatus, error)
	UpdateStatusesByStackId(
		stackID string,
		status entities.DeploymentStatus,
	) error
}

type StackRepository interface {
	CreateStackByTx(
		stack *entities.StackEntity,
		deployments []*entities.DeploymentEntity,
		integration *entities.IntegrationEntity,
	) error
	UpdateStatus(stackId string, status entities.StackStatus, reason string) error
	GetStackByID(stackId string) (*entities.StackEntity, error)
	GetAllStacks() ([]*entities.StackEntity, error)
	GetStackStatus(stackId string) (entities.StackStatus, error)
	UpdateMetadata(
		id string,
		metadata *entities.StackMetadata,
	) error
}

type IntegrationRepository interface {
	CreateIntegration(
		integration *entities.IntegrationEntity,
	) error
	UpdateIntegrationStatus(
		id string,
		status entities.DeploymentStatus,
	) error
	GetInstalledIntegration(
		stackId string,
		integrationType string,
	) (*entities.IntegrationEntity, error)
	GetActiveIntegrations(
		stackId string,
		integrationType string,
	) ([]*entities.IntegrationEntity, error)
	GetIntegration(
		stackId string,
		name string,
	) (*entities.IntegrationEntity, error)
	GetIntegrationById(
		id string,
	) (*entities.IntegrationEntity, error)
	GetIntegrationsByStackID(
		stackID string,
	) ([]*entities.IntegrationEntity, error)
	GetActiveIntegrationsByStackID(
		stackID string,
	) ([]*entities.IntegrationEntity, error)
	UpdateIntegrationsStatusByStackID(
		stackID string,
		status entities.DeploymentStatus,
	) error
	UpdateMetadataAfterInstalled(
		id string,
		metadata *entities.IntegrationInfo,
	) error
	UpdateConfig(
		id string,
		config json.RawMessage,
	) error
}

type TaskManager interface {
	Start()
	AddTask(id string, task entities.Task)
	StopTask(id string)
	Stop()
}

type ThanosStackDeploymentService struct {
	name            string
	deploymentRepo  DeploymentRepository
	stackRepo       StackRepository
	integrationRepo IntegrationRepository
	taskManager     TaskManager
}

func NewThanosService(
	deploymentRepo DeploymentRepository,
	stackRepo StackRepository,
	integrationRepo IntegrationRepository,
	taskManager TaskManager,
) *ThanosStackDeploymentService {
	thanosDeploymentSrv := &ThanosStackDeploymentService{
		name:            "Thanos",
		deploymentRepo:  deploymentRepo,
		stackRepo:       stackRepo,
		integrationRepo: integrationRepo,
		taskManager:     taskManager,
	}

	thanosDeploymentSrv.taskManager.Start()

	return thanosDeploymentSrv
}

func (s *ThanosStackDeploymentService) CreateThanosStack(
	ctx context.Context,
	request dtos.DeployThanosRequest,
) (*entities.Response, error) {
	stackId := uuid.New()
	deploymentPath := utils.GetDeploymentPath(s.name, request.Network, stackId.String())
	request.DeploymentPath = deploymentPath
	config, err := json.Marshal(request)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	stack := &entities.StackEntity{
		ID:             stackId,
		Name:           s.name,
		Network:        request.Network,
		Config:         config,
		DeploymentPath: deploymentPath,
		Status:         entities.StackStatusPending,
	}

	// We install the bridge by default
	bridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    "bridge",
		Status:  string(entities.DeploymentStatusPending),
	}

	deployments, err := getThanosStackDeployments(stackId, &request)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	err = s.stackRepo.CreateStackByTx(stack, deployments, bridgeIntegration)
	if err != nil {
		logger.Error("Failed to create thanos stack", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logger.Info("Stack created", zap.String("stackId", stackId.String()))

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.handleStackDeployment(ctx, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]string{"stackId": stackId.String()},
	}, nil
}

func (s *ThanosStackDeploymentService) StopDeployingThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
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

	if stack.Status != entities.StackStatusDeploying {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deploying, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.StopTask(taskId)
	// Update stacks status to stopping
	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusStopped, "")
	if err != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) ResumeThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
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

	if stack.Status != entities.StackStatusStopped {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not stopped, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.handleStackDeployment(ctx, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) UpdateNetwork(ctx context.Context, stackId uuid.UUID, request dtos.UpdateNetworkRequest) (*entities.Response, error) {
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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
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

	logPath := utils.GetLogPath(stack.ID, "update-network")
	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
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

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusUpdating, "")
	if err != nil {
		logger.Error("failed to update stack status", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("update-network-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		err = thanos.UpdateNetwork(ctx, sdkClient, &request)
		if err != nil {
			logger.Error("failed to update network", zap.Error(err))
		}

		err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeployed, "")
		if err != nil {
			logger.Error("failed to update stack status", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) TerminateThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	// Check if stacks exists
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// Check if stacks is in a valid state to be terminated
	if stack.Status == entities.StackStatusDeploying || stack.Status == entities.StackStatusUpdating ||
		stack.Status == entities.StackStatusTerminating {
		logger.Error(
			"The stacks is still deploying, updating or terminating, please wait for it to finish",
			zap.String("stackId", stackId.String()),
		)
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "The stacks is still deploying, updating or terminating, please wait for it to finish",
			Data:    nil,
		}, nil
	}

	taskId := fmt.Sprintf("terminate-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.handleStackTermination(ctx, stack)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

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
		blockExplorerIntegration := &entities.IntegrationEntity{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Type:    "block-explorer",
			Status:  string(entities.DeploymentStatusInProgress),
			LogPath: logPath,
		}
		err = s.integrationRepo.CreateIntegration(blockExplorerIntegration)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		blockExplorerUrl, err = thanos.InstallBlockExplorer(ctx, sdkClient, &request)
		if err != nil {
			logger.Error("failed to install block explorer", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		if blockExplorerUrl == "" {
			logger.Error("block explorer URL is empty", zap.String("plugin", "block-explorer"))
			return
		}

		logger.Debug("block explorer successfully installed", zap.String("plugin", "block-explorer"), zap.String("url", blockExplorerUrl))
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
			logger.Error("failed to update block explorer integration config", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		blockExplorerMedata := &entities.IntegrationInfo{
			Url: blockExplorerUrl,
		}
		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			blockExplorerIntegration.ID.String(),
			blockExplorerMedata,
		)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", "block-explorer"), zap.Error(err))
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
		integration, err := s.integrationRepo.GetInstalledIntegration(stackId, "block-explorer")
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", "block-explorer"))
			return
		}
		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}
		err = thanos.UninstallBlockExplorer(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install block-explorer", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", "block-explorer"), zap.Error(err))
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
			Message: "Stack is not deployed, yet. Please wait for it to finish",
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

	logPath := utils.GetLogPath(stack.ID, "install-bridge")

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
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
			Type:    "bridge",
			Status:  string(entities.DeploymentStatusInProgress),
			LogPath: logPath,
		}
		err = s.integrationRepo.CreateIntegration(bridgeIntegration)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		bridgeUrl, err = thanos.InstallBridge(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install bridge", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		if bridgeUrl == "" {
			logger.Error("bridge URL is empty", zap.String("plugin", "bridge"))
			return
		}

		logger.Debug("bridge successfully installed", zap.String("plugin", "bridge"), zap.String("url", bridgeUrl))

		// create integration
		bridgeMetadata := &entities.IntegrationInfo{
			Url: bridgeUrl,
		}

		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			bridgeIntegration.ID.String(),
			bridgeMetadata,
		)
		if err != nil {
			logger.Error("failed to update bridge integration metadata", zap.String("plugin", "bridge"), zap.Error(err))
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

		logger.Info("Bridge installed successfully",
			zap.String("stackId", stackId),
			zap.String("bridgeUrl", bridgeUrl),
		)
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
		integration, err := s.integrationRepo.GetInstalledIntegration(stackId, "bridge")
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", "bridge"))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		logger.Info("Uninstalling bridge", zap.String("plugin", "bridge"))

		err = thanos.UninstallBridge(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install bridge", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", "bridge"), zap.Error(err))
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

func (s *ThanosStackDeploymentService) GetAllStacks() (*entities.Response, error) {
	stacks, err := s.stackRepo.GetAllStacks()
	if err != nil {
		logger.Error("failed to get stacks", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"stacks": stacks},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackStatus(stackId uuid.UUID) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
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

	status, err := s.stackRepo.GetStackStatus(stackId.String())
	if err != nil {
		logger.Error("failed to get stack status", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"status": status},
	}, nil
}

func (s *ThanosStackDeploymentService) GetDeployments(
	stackId uuid.UUID,
) (*entities.Response, error) {

	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
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

	deployments, err := s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		logger.Error("failed to get deployments", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"deployments": deployments},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackDeploymentStatus(
	deploymentId uuid.UUID,
) (*entities.Response, error) {
	status, err := s.deploymentRepo.GetDeploymentStatus(deploymentId.String())
	if err != nil {
		logger.Error("failed to get deployment status", zap.String("deploymentId", deploymentId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"status": status},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackDeployment(
	_ uuid.UUID,
	deploymentId uuid.UUID,
) (*entities.Response, error) {
	deployment, err := s.deploymentRepo.GetDeploymentByID(deploymentId.String())
	if err != nil {
		logger.Error("failed to get deployment", zap.String("deploymentId", deploymentId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if deployment == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Deployment not found",
			Data:    nil,
		}, nil
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"deployment": deployment},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackByID(
	stackId uuid.UUID,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
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

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"stack": stack},
	}, nil
}

func (s *ThanosStackDeploymentService) GetIntegrations(
	stackId uuid.UUID,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
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
	integrations, err := s.integrationRepo.GetActiveIntegrationsByStackID(stackId.String())
	if err != nil {
		logger.Error("failed to get integrations", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"integrations": integrations},
	}, nil
}

func (s *ThanosStackDeploymentService) GetIntegration(
	stackId uuid.UUID,
	integrationId uuid.UUID,
) (*entities.Response, error) {
	integration, err := s.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		logger.Error("failed to get integrations", zap.String("stackId", stackId.String()), zap.Error(err))
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

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"integration": integration},
	}, nil
}

// New helper method to handle deployment logic
func (s *ThanosStackDeploymentService) handleStackDeployment(ctx context.Context, stackId uuid.UUID) {
	logger.Info("Updating stacks status to creating", zap.String("stackId", stackId.String()))

	err := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeploying, "")
	if err != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	err = s.deployThanosStack(ctx, stackId)
	if err != nil {
		if err == context.Canceled {
			return
		}
		logger.Error("failed to deploy thanos stacks",
			zap.String("stackId", stackId.String()),
			zap.Error(err))

		// Update stacks status to failed
		updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusFailedToDeploy, err.Error())
		if updateErr != nil {
			logger.Error("failed to update stacks status",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack by id", zap.String("stackId", stackId.String()))
		return
	}

	// Update stacks status to active on success
	updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeployed, "")
	if updateErr != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(updateErr))
	}

	config, err := json.Marshal(stack.Config)
	if err != nil {
		logger.Error("failed to marshal stack config", zap.Error(err))
		return
	}
	var stackConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.Error(err))
		return
	}

	logPath := utils.GetLogPath(stack.ID, "information")
	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	// Get chain information
	chainInformation, err := thanos.ShowChainInformation(ctx, sdkClient)
	if err != nil || chainInformation == nil {
		logger.Error("failed to show chain information", zap.Error(err))
		return
	}

	err = s.stackRepo.UpdateMetadata(stackId.String(), &entities.StackMetadata{
		L2Url:            chainInformation.L2RpcUrl,
		BridgeUrl:        chainInformation.BridgeUrl,
		BlockExplorerUrl: chainInformation.BlockExplorer,
	})
	if err != nil {
		logger.Error("failed to update stack metadata", zap.Error(err))
		return
	}

	bridgeUrl := chainInformation.BridgeUrl
	if bridgeUrl == "" {
		logger.Error("bridge url is empty", zap.String("stackId", stackId.String()))
		return
	}

	// bridgeIntegration
	bridgeIntegration, err := s.integrationRepo.GetIntegration(stackId.String(), "bridge")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "bridge"), zap.Error(err))
		return
	}

	if bridgeIntegration == nil {
		logger.Error("bridge integration not found", zap.String("plugin", "bridge"))
		return
	}

	err = s.integrationRepo.UpdateMetadataAfterInstalled(
		bridgeIntegration.ID.String(),
		&entities.IntegrationInfo{
			Url: bridgeUrl,
		},
	)

	if err != nil {
		logger.Error("failed to create integration", zap.Error(err))
		return
	}

	logger.Info("Thanos stack deployed successfully",
		zap.String("stackId", stackId.String()),
	)
}

func (s *ThanosStackDeploymentService) deployThanosStack(ctx context.Context, stackId uuid.UUID) error {
	statusChan := make(chan entities.DeploymentStatusWithID)
	defer close(statusChan)

	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get stack: %w", err)
	}

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	var deploymentConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &deploymentConfig); err != nil {
		return fmt.Errorf("failed to unmarshal stack config: %w", err)
	}

	deployments, err := s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	if len(deployments) == 0 {
		return fmt.Errorf("no deployments found for stacks %s", stackId)
	}

	// Start a goroutine to handle status updates
	errChan := make(chan error, 1)
	go func() {
		for status := range statusChan {
			if err := s.deploymentRepo.UpdateDeploymentStatus(status.DeploymentID.String(), status.Status); err != nil {
				errChan <- fmt.Errorf("failed to update deployment status: %w", err)
				return
			}
			// If we've processed all deployments successfully, send nil to errChan
			if status.Status == entities.DeploymentStatusCompleted {
				select {
				case errChan <- nil:
				default:
				}
			}
		}
	}()

	for _, deployment := range deployments {
		logger.Info("Processing deployment",
			zap.String("deploymentId", deployment.ID.String()),
			zap.String("status", string(deployment.Status)),
			zap.Int("step", deployment.Step))

		// Skip already completed deployments
		if deployment.Status == entities.DeploymentStatusCompleted {
			continue
		}

		sdkClient, err := thanos.NewThanosSDKClient(
			ctx,
			deployment.LogPath,
			string(stack.Network),
			stack.DeploymentPath,
			deploymentConfig.AwsAccessKey,
			deploymentConfig.AwsSecretAccessKey,
			deploymentConfig.AwsRegion,
		)
		if err != nil {
			logger.Error("failed to create thanos sdk client",
				zap.String("deploymentId", deployment.ID.String()),
				zap.Error(err))
			statusChan <- entities.DeploymentStatusWithID{
				DeploymentID: deployment.ID,
				Status:       entities.DeploymentStatusFailed,
			}
			return err
		}

		// Update status to in-progress before starting deployment
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: deployment.ID,
			Status:       entities.DeploymentStatusInProgress,
		}

		if deployment.Step == 1 {
			var deployL1ContractsConfig dtos.DeployL1ContractsRequest
			if err := json.Unmarshal(deployment.Config, &deployL1ContractsConfig); err != nil {
				return fmt.Errorf("failed to unmarshal deployment config: %w", err)
			}

			if err := thanos.DeployL1Contracts(ctx, sdkClient, &deployL1ContractsConfig); err != nil {
				if err == context.Canceled {
					logger.Info("deployment cancelled",
						zap.String("deploymentId", deployment.ID.String()),
						zap.Int("step", deployment.Step))
					statusChan <- entities.DeploymentStatusWithID{
						DeploymentID: deployment.ID,
						Status:       entities.DeploymentStatusStopped,
					}
					return err
				}
				logger.Error("deployment failed",
					zap.String("deploymentId", deployment.ID.String()),
					zap.Int("step", deployment.Step),
					zap.Error(err))
				statusChan <- entities.DeploymentStatusWithID{
					DeploymentID: deployment.ID,
					Status:       entities.DeploymentStatusFailed,
				}
				return err
			}
			statusChan <- entities.DeploymentStatusWithID{
				DeploymentID: deployment.ID,
				Status:       entities.DeploymentStatusCompleted,
			}
		} else if deployment.Step == 2 {
			var deployAwsInfraConfig dtos.DeployThanosAWSInfraRequest
			if err := json.Unmarshal(deployment.Config, &deployAwsInfraConfig); err != nil {
				return fmt.Errorf("failed to unmarshal deployment config: %w", err)
			}

			if err := thanos.DeployAWSInfrastructure(ctx, sdkClient, &deployAwsInfraConfig); err != nil {
				if err == context.Canceled {
					logger.Info("deployment cancelled",
						zap.String("deploymentId", deployment.ID.String()),
						zap.Int("step", deployment.Step))
					statusChan <- entities.DeploymentStatusWithID{
						DeploymentID: deployment.ID,
						Status:       entities.DeploymentStatusStopped,
					}
					return err
				}
				logger.Error("deployment failed",
					zap.String("deploymentId", deployment.ID.String()),
					zap.Int("step", deployment.Step),
					zap.Error(err))
				statusChan <- entities.DeploymentStatusWithID{
					DeploymentID: deployment.ID,
					Status:       entities.DeploymentStatusFailed,
				}
				return err
			}
			statusChan <- entities.DeploymentStatusWithID{
				DeploymentID: deployment.ID,
				Status:       entities.DeploymentStatusCompleted,
			}
		}

	}

	// Wait for final status update
	return <-errChan
}

func (s *ThanosStackDeploymentService) handleStackTermination(ctx context.Context, stack *entities.StackEntity) {
	// Check if stacks exists
	if stack == nil {
		logger.Error("stack not found")
		return
	}

	stackId := stack.ID

	stackConfig := dtos.DeployThanosRequest{}
	err := json.Unmarshal(stack.Config, &stackConfig)
	if err != nil {
		logger.Error("failed to unmarshal stacks config",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusFailedToTerminate, err.Error()); updateErr != nil {
			logger.Error("failed to update stacks status after unmarshal error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	logPath := utils.GetLogPath(stack.ID, "destroy")

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusTerminating, "")
	if err != nil {
		logger.Error("failed to update stacks status after destroy error",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	err = thanos.DestroyAWSInfrastructure(ctx, sdkClient)
	if err != nil {
		logger.Error("failed to destroy AWS infrastructure",
			zap.String("stackId", stackId.String()),
			zap.Error(err))

		updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusFailedToTerminate, err.Error())
		if updateErr != nil {
			logger.Error("failed to update stacks status after destroy error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusTerminated, "")
	if err != nil {
		logger.Error("failed to update stacks status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	err = s.deploymentRepo.UpdateStatusesByStackId(
		stackId.String(),
		entities.DeploymentStatusTerminated,
	)
	if err != nil {
		logger.Error("failed to update deployments status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	// Update integrations status to terminated
	err = s.integrationRepo.UpdateIntegrationsStatusByStackID(
		stackId.String(),
		entities.DeploymentStatusTerminated,
	)
	if err != nil {
		logger.Error("failed to update integrations status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	logger.Info(
		"AWS infrastructure destroyed successfully",
		zap.String("stackId", stackId.String()),
	)
}

func getThanosStackDeployments(
	stackId uuid.UUID,
	config *dtos.DeployThanosRequest,
) ([]*entities.DeploymentEntity, error) {
	deployments := make([]*entities.DeploymentEntity, 0)
	l1ContractDeploymentID := uuid.New()
	l1ContractDeploymentLogPath := utils.GetLogPath(stackId, "deploy-l1-contracts")
	l1ContractDeploymentConfig, err := json.Marshal(dtos.DeployL1ContractsRequest{
		L1RpcUrl:                 config.L1RpcUrl,
		L2BlockTime:              config.L2BlockTime,
		BatchSubmissionFrequency: config.BatchSubmissionFrequency,
		OutputRootFrequency:      config.OutputRootFrequency,
		ChallengePeriod:          config.ChallengePeriod,
		AdminAccount:             config.AdminAccount,
		SequencerAccount:         config.SequencerAccount,
		BatcherAccount:           config.BatcherAccount,
		ProposerAccount:          config.ProposerAccount,
	})
	if err != nil {
		return nil, err
	}
	l1ContractDeployment := &entities.DeploymentEntity{
		ID:      l1ContractDeploymentID,
		StackID: &stackId,
		Step:    1,
		Status:  entities.DeploymentStatusPending,
		LogPath: l1ContractDeploymentLogPath,
		Config:  l1ContractDeploymentConfig,
	}
	deployments = append(deployments, l1ContractDeployment)

	thanosInfrastructureDeploymentID := uuid.New()
	thanosInfrastructureDeploymentLogPath := utils.GetLogPath(
		stackId,
		"deploy-thanos-aws-infra",
	)
	thanosInfrastructureDeploymentConfig, err := json.Marshal(dtos.DeployThanosAWSInfraRequest{
		ChainName:   config.ChainName,
		L1BeaconUrl: config.L1BeaconUrl,
	})
	if err != nil {
		return nil, err
	}
	thanosInfrastructureDeployment := &entities.DeploymentEntity{
		ID:      thanosInfrastructureDeploymentID,
		StackID: &stackId,
		Step:    2,
		Status:  entities.DeploymentStatusPending,
		LogPath: thanosInfrastructureDeploymentLogPath,
		Config:  thanosInfrastructureDeploymentConfig,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}
