package services

import (
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
}

type TaskManager interface {
	Start()
	AddTask(task entities.Task)
	Stop()
}

type ThanosStackDeploymentService struct {
	name           string
	deploymentRepo DeploymentRepository
	stackRepo      StackRepository
	taskManager    TaskManager
}

func NewThanosService(
	deploymentRepo DeploymentRepository,
	stackRepo StackRepository,
	taskManager TaskManager,
) *ThanosStackDeploymentService {
	thanosDeploymentSrv := &ThanosStackDeploymentService{
		name:           "Thanos",
		deploymentRepo: deploymentRepo,
		stackRepo:      stackRepo,
		taskManager:    taskManager,
	}

	thanosDeploymentSrv.taskManager.Start()

	return thanosDeploymentSrv
}

func (s *ThanosStackDeploymentService) CreateThanosStack(
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
		s.handleStackDeployment(stackId)
	})

	return stackId, nil
}

// New helper method to handle deployment logic
func (s *ThanosStackDeploymentService) handleStackDeployment(stackId uuid.UUID) {
	logger.Info("Updating stacks status to creating", zap.String("stackId", stackId.String()))

	err := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusDeploying, "")
	if err != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	if err := s.deployThanosStack(stackId); err != nil {
		logger.Error("failed to deploy thanos stacks",
			zap.String("stackId", stackId.String()),
			zap.Error(err))

		// Update stacks status to failed
		if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToDeploy, err.Error()); updateErr != nil {
			logger.Error("failed to update stacks status",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
	} else {
		// Update stacks status to active on success
		if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StatusDeployed, ""); updateErr != nil {
			logger.Error("failed to update stacks status",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
	}
}

func (s *ThanosStackDeploymentService) deployThanosStack(stackId uuid.UUID) error {
	statusChan := make(chan entities.DeploymentStatusWithID)
	defer close(statusChan)

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

		deploymentConfig := dtos.DeployThanosRequest{}
		if err := json.Unmarshal(deployment.Config, &deploymentConfig); err != nil {
			return fmt.Errorf("failed to unmarshal deployment config: %w", err)
		}

		// Update status to in-progress before starting deployment
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: deployment.ID,
			Status:       entities.DeploymentStatusInProgress,
		}

		var err error
		if deployment.Step == 1 {
			err = s.deployL1Contracts(statusChan, deployment.ID, dtos.DeployL1ContractsRequest{
				Network:                  deploymentConfig.Network,
				L1RpcUrl:                 deploymentConfig.L1RpcUrl,
				L2BlockTime:              deploymentConfig.L2BlockTime,
				BatchSubmissionFrequency: deploymentConfig.BatchSubmissionFrequency,
				OutputRootFrequency:      deploymentConfig.OutputRootFrequency,
				ChallengePeriod:          deploymentConfig.ChallengePeriod,
				AdminAccount:             deploymentConfig.AdminAccount,
				SequencerAccount:         deploymentConfig.SequencerAccount,
				BatcherAccount:           deploymentConfig.BatcherAccount,
				ProposerAccount:          deploymentConfig.ProposerAccount,
				DeploymentPath:           deploymentConfig.DeploymentPath,
				LogPath:                  deployment.LogPath,
			})
		} else if deployment.Step == 2 {
			err = s.deployThanosAWSInfra(statusChan, deployment.ID, dtos.DeployThanosAWSInfraRequest{
				ChainName:          deploymentConfig.ChainName,
				Network:            string(deploymentConfig.Network),
				L1BeaconUrl:        deploymentConfig.L1BeaconUrl,
				AwsAccessKey:       deploymentConfig.AwsAccessKey,
				AwsSecretAccessKey: deploymentConfig.AwsSecretAccessKey,
				AwsRegion:          deploymentConfig.AwsRegion,
				DeploymentPath:     deploymentConfig.DeploymentPath,
				LogPath:            deployment.LogPath,
			})
		}

		if err != nil {
			logger.Error("deployment failed",
				zap.String("deploymentId", deployment.ID.String()),
				zap.Int("step", deployment.Step),
				zap.Error(err))
			return err
		}
	}

	// Wait for final status update
	return <-errChan
}

func (s *ThanosStackDeploymentService) deployL1Contracts(
	statusChan chan entities.DeploymentStatusWithID,
	deploymentID uuid.UUID,
	request dtos.DeployL1ContractsRequest,
) error {
	if err := thanos.DeployL1Contracts(&request); err != nil {
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: deploymentID,
			Status:       entities.DeploymentStatusFailed,
		}
		return err
	}
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: deploymentID,
		Status:       entities.DeploymentStatusCompleted,
	}
	return nil
}

func (s *ThanosStackDeploymentService) deployThanosAWSInfra(
	statusChan chan entities.DeploymentStatusWithID,
	deploymentID uuid.UUID,
	request dtos.DeployThanosAWSInfraRequest,
) error {
	if err := thanos.DeployAWSInfrastructure(&request); err != nil {
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: deploymentID,
			Status:       entities.DeploymentStatusFailed,
		}
		return err
	}
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: deploymentID,
		Status:       entities.DeploymentStatusCompleted,
	}
	return nil
}

func (s *ThanosStackDeploymentService) ResumeThanosStack(stackId uuid.UUID) error {
	s.taskManager.AddTask(func() {
		s.handleStackDeployment(stackId)
	})
	return nil
}

func (s *ThanosStackDeploymentService) TerminateThanosStack(stackId uuid.UUID) error {
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
		s.handleStackTermination(stackId)
	})

	return nil
}

func (s *ThanosStackDeploymentService) handleStackTermination(stackId uuid.UUID) {
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

	if err := thanos.DestroyAWSInfrastructure(&dtos.TerminateThanosRequest{
		Network:            string(stack.Network),
		AwsAccessKey:       stackConfig.AwsAccessKey,
		AwsSecretAccessKey: stackConfig.AwsSecretAccessKey,
		AwsRegion:          stackConfig.AwsRegion,
		DeploymentPath:     stack.DeploymentPath,
		LogPath:            logPath,
	}); err != nil {
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
		Network:                  config.Network,
		L1RpcUrl:                 config.L1RpcUrl,
		L2BlockTime:              config.L2BlockTime,
		BatchSubmissionFrequency: config.BatchSubmissionFrequency,
		OutputRootFrequency:      config.OutputRootFrequency,
		ChallengePeriod:          config.ChallengePeriod,
		AdminAccount:             config.AdminAccount,
		SequencerAccount:         config.SequencerAccount,
		BatcherAccount:           config.BatcherAccount,
		ProposerAccount:          config.ProposerAccount,
		DeploymentPath:           deploymentPath,
		LogPath:                  l1ContractDeploymentLogPath,
	})
	if err != nil {
		return nil, err
	}
	l1ContractDeployment := &entities.DeploymentEntity{
		ID:             l1ContractDeploymentID,
		StackID:        &stackId,
		IntegrationID:  nil,
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
		ChainName:          config.ChainName,
		Network:            string(config.Network),
		L1BeaconUrl:        config.L1BeaconUrl,
		AwsAccessKey:       config.AwsAccessKey,
		AwsSecretAccessKey: config.AwsSecretAccessKey,
		AwsRegion:          config.AwsRegion,
		DeploymentPath:     deploymentPath,
		LogPath:            thanosInfrastructureDeploymentLogPath,
	})
	if err != nil {
		return nil, err
	}
	thanosInfrastructureDeployment := &entities.DeploymentEntity{
		ID:             thanosInfrastructureDeploymentID,
		StackID:        &stackId,
		IntegrationID:  nil,
		Step:           2,
		Status:         entities.DeploymentStatusPending,
		LogPath:        thanosInfrastructureDeploymentLogPath,
		Config:         thanosInfrastructureDeploymentConfig,
		DeploymentPath: deploymentPath,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}
