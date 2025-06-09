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
}

type StackRepository interface {
	CreateStackByTx(stack *entities.StackEntity, deployments []*entities.DeploymentEntity) error
	UpdateStatus(stackId string, status entities.Status, reason string) error
	GetStackByID(stackId string) (*entities.StackEntity, error)
	GetAllStacks() ([]*entities.StackEntity, error)
	GetStackStatus(stackId string) (entities.Status, error)
	UpdateMetadata(
		id string,
		metadata json.RawMessage,
	) error
}

type IntegrationRepository interface {
	CreateIntegration(
		integration *entities.Integration,
	) error
	UpdateIntegrationStatus(
		id string,
		status entities.Status,
	) error
	GetIntegration(
		stackId string,
		name string,
	) (*entities.Integration, error)
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
		Status:         entities.StatusPending,
	}

	deployments, err := getThanosStackDeployments(stackId, &request, deploymentPath)
	if err != nil {
		return uuid.Nil, err
	}

	err = s.stackRepo.CreateStackByTx(stack, deployments)
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

// New helper method to handle deployment logic
func (s *ThanosStackDeploymentService) handleStackDeployment(ctx context.Context, stackId uuid.UUID) {
	logger.Info("Updating stacks status to creating", zap.String("stackId", stackId.String()))

	err := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusDeploying, "")
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
		updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToDeploy, err.Error())
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
	updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusDeployed, "")
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

	metadata, err := json.Marshal(chainInformation)
	if err != nil {
		logger.Error("failed to marshal chain information", zap.Error(err))
		return
	}

	err = s.stackRepo.UpdateMetadata(stackId.String(), metadata)
	if err != nil {
		logger.Error("failed to update stack metadata", zap.Error(err))
		return
	}

	bridgeUrl := chainInformation.BridgeUrl
	if bridgeUrl == "" {
		logger.Error("bridge url is empty", zap.String("stackId", stackId.String()))
		return
	}

	// create integration by default(bridge)
	err = s.integrationRepo.CreateIntegration(&entities.Integration{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Name:    "bridge",
		Status:  string(entities.DeploymentStatusCompleted),
		Info:    json.RawMessage(fmt.Sprintf(`{"url": "%s"}`, bridgeUrl)),
		Config:  nil,
	})

	if err != nil {
		logger.Error("failed to create integration", zap.Error(err))
		return
	}
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
	if stack.Status == entities.StatusDeploying || stack.Status == entities.StatusUpdating ||
		stack.Status == entities.StatusTerminating {
		logger.Error(
			"The stacks is still deploying, updating or terminating, please wait for it to finish",
			zap.String("stackId", stackId.String()),
		)
		return fmt.Errorf(
			"the stacks is still deploying, updating or terminating, please wait for it to finish",
		)
	}

	s.taskManager.AddTask(func() {
		s.handleStackTermination(ctx, stackId)
	})

	return nil
}

func (s *ThanosStackDeploymentService) handleStackTermination(ctx context.Context, stackId uuid.UUID) {
	// Check if stacks exists
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error(
			"failed to get stacks",
			zap.String("stackId", stackId.String()),
			zap.Error(err),
		)
		return
	}

	// Update stacks status to terminating
	if err := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusTerminating, ""); err != nil {
		logger.Error("failed to update stacks status to terminating",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stacks config",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToTerminate, err.Error()); updateErr != nil {
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

	if err := thanos.DestroyAWSInfrastructure(ctx, sdkClient); err != nil {
		logger.Error("failed to destroy AWS infrastructure",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToTerminate, err.Error()); updateErr != nil {
			logger.Error("failed to update stacks status after destroy error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	if err := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusTerminated, ""); err != nil {
		logger.Error("failed to update stacks status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	deployments, err := s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		logger.Error("failed to get deployments",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	for _, deployment := range deployments {
		if err := s.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentStatus(entities.StatusPending)); err != nil {
			logger.Error("failed to update deployment status",
				zap.String("deploymentId", deployment.ID.String()),
				zap.String("stackId", stackId.String()),
				zap.Error(err))
			// Continue updating other deployments even if one fails
			continue
		}
	}

	logger.Info(
		"AWS infrastructure destroyed successfully",
		zap.String("stackId", stackId.String()),
	)
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

	if stack == nil {
		return fmt.Errorf("stack %s not found", stackId)
	}

	// check if block explorer is already installed
	integration, err := s.integrationRepo.GetIntegration(stackId, "block-explorer")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "block-explorer"), zap.Error(err))
		return err
	}

	if integration != nil {
		logger.Error("block explorer is already installed", zap.String("plugin", "block-explorer"))
		return fmt.Errorf("block explorer is already installed")
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
		b, err := json.Marshal(request)
		if err != nil {
			logger.Error("failed to marshal block explorer config", zap.Error(err))
			return
		}
		err = s.integrationRepo.CreateIntegration(&entities.Integration{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Name:    "block-explorer",
			Status:  string(entities.DeploymentStatusCompleted),
			Info:    json.RawMessage(fmt.Sprintf(`{"url": "%s"}`, blockExplorerUrl)),
			Config:  b,
			LogPath: logPath,
		})
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", "block-explorer"), zap.Error(err))
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
		err = thanos.UninstallBlockExplorer(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install block-explorer", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		integration, err := s.integrationRepo.GetIntegration(stackId, "block-explorer")
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", "block-explorer"), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", "block-explorer"))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.StatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", "block-explorer"), zap.Error(err))
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

	// check if block explorer is already installed
	integration, err := s.integrationRepo.GetIntegration(stackId, "bridge")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "bridge"), zap.Error(err))
		return err
	}

	if integration != nil {
		logger.Error("bridge is already installed", zap.String("plugin", "bridge"))
		return fmt.Errorf("bridge is already installed")
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
		err = s.integrationRepo.CreateIntegration(&entities.Integration{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Name:    "bridge",
			Status:  string(entities.DeploymentStatusCompleted),
			Info:    json.RawMessage(fmt.Sprintf(`{"url": "%s"}`, bridgeUrl)),
			Config:  nil,
			LogPath: logPath,
		})
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}
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
		err = thanos.UninstallBridge(ctx, sdkClient)
		if err != nil {
			logger.Error("failed to install bridge", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		integration, err := s.integrationRepo.GetIntegration(stackId, "bridge")
		if err != nil {
			logger.Error("failed to get integration", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}

		if integration == nil {
			logger.Error("integration not found", zap.String("plugin", "bridge"))
			return
		}

		err = s.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.StatusTerminated)
		if err != nil {
			logger.Error("failed to update integration", zap.String("plugin", "bridge"), zap.Error(err))
			return
		}
	})

	return nil
}

func (s *ThanosStackDeploymentService) GetAllStacks() ([]*entities.StackEntity, error) {
	return s.stackRepo.GetAllStacks()
}

func (s *ThanosStackDeploymentService) GetStackStatus(stackId uuid.UUID) (entities.Status, error) {
	return s.stackRepo.GetStackStatus(stackId.String())
}

func (s *ThanosStackDeploymentService) GetStackDeployments(
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
		ID:             l1ContractDeploymentID,
		StackID:        &stackId,
		Step:           1,
		Status:         entities.DeploymentStatusPending,
		LogPath:        l1ContractDeploymentLogPath,
		Config:         l1ContractDeploymentConfig,
		DeploymentPath: deploymentPath,
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
		ID:             thanosInfrastructureDeploymentID,
		StackID:        &stackId,
		Step:           2,
		Status:         entities.DeploymentStatusPending,
		LogPath:        thanosInfrastructureDeploymentLogPath,
		Config:         thanosInfrastructureDeploymentConfig,
		DeploymentPath: deploymentPath,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}
