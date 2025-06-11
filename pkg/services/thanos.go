package services

import (
	"context"
	"encoding/json"
	"fmt"

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
	AddTask(task entities.Task)
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
) (uuid.UUID, error) {
	stackId := uuid.New()
	deploymentPath := utils.GetDeploymentPath(s.name, request.Network, stackId.String())
	request.DeploymentPath = deploymentPath
	config, err := json.Marshal(request)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal config: %w", err)
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

	deployments, err := getThanosStackDeployments(stackId, &request, deploymentPath)
	if err != nil {
		return uuid.Nil, err
	}

	err = s.stackRepo.CreateStackByTx(stack, deployments, bridgeIntegration)
	if err != nil {
		logger.Error("Failed to create thanos stack", zap.Error(err))
		return uuid.Nil, err
	}

	logger.Info("Stack created", zap.String("stackId", stackId.String()))

	s.taskManager.AddTask(func() {
		s.handleStackDeployment(ctx, stackId)
	})

	return stackId, nil
}

func (s *ThanosStackDeploymentService) ResumeThanosStack(ctx context.Context, stackId uuid.UUID) error {
	s.taskManager.AddTask(func() {
		s.handleStackDeployment(ctx, stackId)
	})
	return nil
}

func (s *ThanosStackDeploymentService) TerminateThanosStack(ctx context.Context, stackId uuid.UUID) error {
	// Check if stacks exists
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get stacks: %w", err)
	}

	// Check if stacks is in a valid state to be terminated
	if stack.Status == entities.StackStatusDeploying || stack.Status == entities.StackStatusUpdating ||
		stack.Status == entities.StackStatusTerminating {
		logger.Error(
			"The stacks is still deploying, updating or terminating, please wait for it to finish",
			zap.String("stackId", stackId.String()),
		)
		return fmt.Errorf(
			"the stacks is still deploying, updating or terminating, please wait for it to finish",
		)
	}

	s.taskManager.AddTask(func() {
		s.handleStackTermination(ctx, stack)
	})

	return nil
}

func (s *ThanosStackDeploymentService) InstallBlockExplorer(ctx context.Context, stackId string, request dtos.InstallBlockExplorerRequest) error {
	if err := request.Validate(); err != nil {
		logger.Error("invalid block explorer request", zap.Error(err))
		return err
	}

	stack, err := s.stackRepo.GetStackByID(stackId)
	if err != nil {
		return err
	}

	if stack.Status != entities.StackStatusDeployed {
		return fmt.Errorf("stack %s is not deployed, yet. Please wait for it to finish", stackId)
	}

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	// check if block explorer is already in non-terminated state
	integrations, err := s.integrationRepo.GetActiveIntegrations(stackId, "block-explorer")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "block-explorer"), zap.Error(err))
		return err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active block explorer", zap.String("plugin", "block-explorer"))
		return fmt.Errorf("there is already an active block explorer")
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return err
	}

	var (
		blockExplorerUrl string
	)

	logPath := utils.GetPluginLogPath(stack.ID, "block-explorer")
	sdkClient, err := thanos.NewThanosSDKClient(
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
		return err
	}

	s.taskManager.AddTask(func() {
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

	return nil
}

func (s *ThanosStackDeploymentService) UninstallBlockExplorer(ctx context.Context, stackId string) error {
	stack, err := s.stackRepo.GetStackByID(stackId)
	if err != nil {
		return err
	}

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return err
	}

	logPath := utils.GetPluginLogPath(stack.ID, "uninstall-block-explorer")
	sdkClient, err := thanos.NewThanosSDKClient(
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
		return err
	}

	s.taskManager.AddTask(func() {
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

	return nil
}

func (s *ThanosStackDeploymentService) InstallBridge(ctx context.Context, stackId string) error {
	stack, err := s.stackRepo.GetStackByID(stackId)
	if err != nil {
		return err
	}

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	if stack.Status != entities.StackStatusDeployed {
		return fmt.Errorf("stack %s is not deployed, yet. Please wait for it to finish", stackId)
	}

	// check if bridge is already in non-terminated state
	integrations, err := s.integrationRepo.GetActiveIntegrations(stackId, "bridge")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "bridge"), zap.Error(err))
		return err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active bridge", zap.String("plugin", "bridge"))
		return fmt.Errorf("there is already an active bridge")
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return err
	}

	var (
		bridgeUrl string
	)

	logPath := utils.GetPluginLogPath(stack.ID, "install-bridge")

	sdkClient, err := thanos.NewThanosSDKClient(
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
		return err
	}

	s.taskManager.AddTask(func() {
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

	return nil
}

func (s *ThanosStackDeploymentService) UninstallBridge(ctx context.Context, stackId string) error {
	stack, err := s.stackRepo.GetStackByID(stackId)
	if err != nil {
		return err
	}

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return err
	}

	logPath := utils.GetPluginLogPath(stack.ID, "uninstall-bridge")

	sdkClient, err := thanos.NewThanosSDKClient(
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
		return err
	}

	s.taskManager.AddTask(func() {
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

	return nil
}

func (s *ThanosStackDeploymentService) GetAllStacks() ([]*entities.StackEntity, error) {
	return s.stackRepo.GetAllStacks()
}

func (s *ThanosStackDeploymentService) GetStackStatus(stackId uuid.UUID) (entities.StackStatus, error) {
	return s.stackRepo.GetStackStatus(stackId.String())
}

func (s *ThanosStackDeploymentService) GetDeployments(
	stackId uuid.UUID,
) ([]*entities.DeploymentEntity, error) {
	return s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
}

func (s *ThanosStackDeploymentService) GetStackDeploymentStatus(
	deploymentId uuid.UUID,
) (entities.DeploymentStatus, error) {
	return s.deploymentRepo.GetDeploymentStatus(deploymentId.String())
}

func (s *ThanosStackDeploymentService) GetStackDeployment(
	_ uuid.UUID,
	deploymentId uuid.UUID,
) (*entities.DeploymentEntity, error) {
	return s.deploymentRepo.GetDeploymentByID(deploymentId.String())
}

func (s *ThanosStackDeploymentService) GetStackByID(
	stackId uuid.UUID,
) (*entities.StackEntity, error) {
	return s.stackRepo.GetStackByID(stackId.String())
}

func (s *ThanosStackDeploymentService) GetIntegrations(
	stackId uuid.UUID,
) ([]*entities.IntegrationEntity, error) {
	integrations, err := s.integrationRepo.GetActiveIntegrationsByStackID(stackId.String())
	if err != nil {
		logger.Error("failed to get integrations", zap.String("stackId", stackId.String()), zap.Error(err))
		return nil, err
	}
	return integrations, nil
}

func (s *ThanosStackDeploymentService) GetIntegration(
	stackId uuid.UUID,
	integrationId uuid.UUID,
) (*entities.IntegrationEntity, error) {
	integration, err := s.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		logger.Error("failed to get integrations", zap.String("stackId", stackId.String()), zap.Error(err))
		return nil, err
	}
	return integration, nil
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

	logPath := utils.GetInformationLogPath(stack.ID)
	sdkClient, err := thanos.NewThanosSDKClient(
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

	logPath := utils.GetDestroyLogPath(stack.ID)

	sdkClient, err := thanos.NewThanosSDKClient(
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
	deploymentPath string,
) ([]*entities.DeploymentEntity, error) {
	deployments := make([]*entities.DeploymentEntity, 0)
	l1ContractDeploymentID := uuid.New()
	l1ContractDeploymentLogPath := utils.GetDeploymentLogPath(stackId, l1ContractDeploymentID)
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
	thanosInfrastructureDeploymentLogPath := utils.GetDeploymentLogPath(
		stackId,
		thanosInfrastructureDeploymentID,
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
