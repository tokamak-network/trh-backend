package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"trh-backend/internal/consts"
	"trh-backend/internal/logger"
	"trh-backend/internal/utils"
	"trh-backend/pkg/domain/entities"
	"trh-backend/pkg/domain/services"
	postgresRepositories "trh-backend/pkg/infrastructure/postgres/repositories"
	trh_sdk_infrastructure "trh-backend/pkg/infrastructure/trh_sdk"
	"trh-backend/pkg/interfaces/api/dtos"

	"go.uber.org/zap"

	"github.com/google/uuid"
	trh_sdk_aws "github.com/tokamak-network/trh-sdk/pkg/cloud-provider/aws"
	trh_sdk_types "github.com/tokamak-network/trh-sdk/pkg/types"
	trh_sdk_utils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"gorm.io/gorm"
)

var (
	chainNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9 ]*$`)
)

type ThanosService struct {
	db                  *gorm.DB
	ThanosDomainService *services.ThanosDomainService
}

func NewThanosService(db *gorm.DB, thanosDomainService *services.ThanosDomainService) *ThanosService {
	return &ThanosService{db: db, ThanosDomainService: thanosDomainService}
}

func (s *ThanosService) CreateThanosStack(request dtos.DeployThanosRequest) (uuid.UUID, error) {

	stackId := uuid.New()
	stackName := "thanos"
	deploymentPath := utils.GetDeploymentPath(stackName, request.Network, stackId.String())
	config, err := json.Marshal(request)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	stack := entities.StackEntity{
		ID:             stackId,
		Name:           stackName,
		Network:        request.Network,
		Config:         config,
		DeploymentPath: deploymentPath,
		Status:         entities.StatusPending,
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return uuid.Nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil || err != nil {
			tx.Rollback()
		}
	}()

	stackRepo := postgresRepositories.NewStackPostgresRepository(tx)
	err = stackRepo.CreateStack(&stack)
	if err != nil {
		return uuid.Nil, err
	}

	deployments, err := s.ThanosDomainService.GetThanosStackDeployments(stackId, &request, deploymentPath)
	if err != nil {
		return uuid.Nil, err
	}

	deploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(tx)
	if err != nil {
		return uuid.Nil, err
	}

	for _, deployment := range deployments {
		err = deploymentRepo.CreateDeployment(&deployment)
		if err != nil {
			tx.Rollback()
			return uuid.Nil, err
		}
	}

	err = tx.Commit().Error
	if err != nil {
		return uuid.Nil, err
	}
	logger.Info("Stack created", zap.String("stackId", stackId.String()))
	go s.handleStackDeployment(stackId)

	return stackId, nil
}

// New helper method to handle deployment logic
func (s *ThanosService) handleStackDeployment(stackId uuid.UUID) {
	logger.Info("Updating stack status to creating", zap.String("stackId", stackId.String()))
	mainStackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	mainDeploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(s.db)

	err := mainStackRepo.UpdateStatus(stackId.String(), entities.StatusDeploying)
	if err != nil {
		logger.Error("failed to update stack status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	if err := s.DeployThanosStack(stackId, mainStackRepo, mainDeploymentRepo); err != nil {
		logger.Error("failed to deploy thanos stack",
			zap.String("stackId", stackId.String()),
			zap.Error(err))

		// Update stack status to failed
		if updateErr := mainStackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToDeploy); updateErr != nil {
			logger.Error("failed to update stack status",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
	} else {
		// Update stack status to active on success
		if updateErr := mainStackRepo.UpdateStatus(stackId.String(), entities.StatusDeployed); updateErr != nil {
			logger.Error("failed to update stack status",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
	}
}

func (s *ThanosService) DeployThanosStack(stackId uuid.UUID, stackRepo *postgresRepositories.StackPostgresRepository, deploymentRepo *postgresRepositories.DeploymentPostgresRepository) error {

	statusChan := make(chan entities.DeploymentStatusWithID)
	defer close(statusChan)

	_, err := stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get stack: %w", err)
	}

	deployments, err := deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get deployments: %w", err)
	}

	if len(deployments) == 0 {
		return fmt.Errorf("no deployments found for stack %s", stackId)
	}

	// Start a goroutine to handle status updates
	errChan := make(chan error, 1)
	go func() {
		for status := range statusChan {
			if err := deploymentRepo.UpdateDeploymentStatus(status.DeploymentID.String(), status.Status); err != nil {
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
			err = s.DeployL1Contracts(statusChan, deployment.ID, dtos.DeployL1ContractsRequest{
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
			err = s.DeployThanosAWSInfra(statusChan, deployment.ID, dtos.DeployThanosAWSInfraRequest{
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

func (s *ThanosService) DeployL1Contracts(statusChan chan entities.DeploymentStatusWithID, deploymentID uuid.UUID, request dtos.DeployL1ContractsRequest) error {
	thanosStack := trh_sdk_infrastructure.NewThanosStack()
	if err := thanosStack.DeployL1Contracts(&request); err != nil {
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

func (s *ThanosService) DeployThanosAWSInfra(statusChan chan entities.DeploymentStatusWithID, deploymentID uuid.UUID, request dtos.DeployThanosAWSInfraRequest) error {
	thanosStack := trh_sdk_infrastructure.NewThanosStack()
	if err := thanosStack.DeployAWSInfrastructure(&request); err != nil {
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

func (s *ThanosService) ValidateThanosRequest(request dtos.DeployThanosRequest) error {
	if request.Network == entities.DeploymentNetworkLocalDevnet {
		return errors.New("local devnet is not supported yet")
	}

	// Validate Chain Name
	if !chainNameRegex.MatchString(request.ChainName) {
		logger.Error("invalid chainName", zap.String("chainName", request.ChainName))
		return errors.New("invalid chain name, chain name must contain only letters (a-z, A-Z), numbers (0-9), spaces. Special characters are not allowed")
	}

	// Validate L1 RPC URL
	if !trh_sdk_utils.IsValidL1RPC(request.L1RpcUrl) {
		logger.Error("invalid l1RpcUrl", zap.String("l1RpcUrl", request.L1RpcUrl))
		return errors.New("invalid l1RpcUrl")
	}

	// Validate L1 Beacon URL
	if !trh_sdk_utils.IsValidBeaconURL(request.L1BeaconUrl) {
		logger.Error("invalid l1BeaconUrl", zap.String("l1BeaconUrl", request.L1BeaconUrl))
		return errors.New("invalid l1BeaconUrl")
	}

	// Validate AWS Access Key
	if !trh_sdk_utils.IsValidAWSAccessKey(request.AwsAccessKey) {
		logger.Error("invalid awsAccessKey", zap.String("awsAccessKey", request.AwsAccessKey))
		return errors.New("invalid awsAccessKey")
	}

	// Validate AWS Secret Key
	if !trh_sdk_utils.IsValidAWSSecretKey(request.AwsSecretAccessKey) {
		logger.Error("invalid awsSecretKey", zap.String("awsSecretAccessKey", request.AwsSecretAccessKey))
		return errors.New("invalid awsSecretKey")
	}

	// Validate AWS Region
	if !trh_sdk_aws.IsAvailableRegion(request.AwsAccessKey, request.AwsSecretAccessKey, request.AwsRegion) {
		logger.Error("invalid awsRegion", zap.String("awsRegion", request.AwsRegion))
		return errors.New("invalid awsRegion")
	}

	// Validate Chain Config
	chainID, err := utils.GetChainIDFromRPC(request.L1RpcUrl)
	if err != nil {
		logger.Error("invalid chainId", zap.String("chainId", err.Error()))
		return errors.New("invalid chainId")
	}
	chainConfig := trh_sdk_types.ChainConfiguration{
		BatchSubmissionFrequency: uint64(request.BatchSubmissionFrequency),
		OutputRootFrequency:      uint64(request.OutputRootFrequency),
		ChallengePeriod:          uint64(request.ChallengePeriod),
		L2BlockTime:              uint64(request.L2BlockTime),
		L1BlockTime:              consts.L1_BLOCK_TIME,
	}

	err = chainConfig.Validate(chainID)
	if err != nil {
		logger.Error("invalid chainConfig", zap.String("chainConfig", err.Error()))
		return err
	}

	return nil
}

func (s *ThanosService) ResumeThanosStack(stackId uuid.UUID) error {
	go s.handleStackDeployment(stackId)
	return nil
}

func (s *ThanosService) TerminateThanosStack(stackId uuid.UUID) error {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)

	// Check if stack exists
	stack, err := stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return fmt.Errorf("failed to get stack: %w", err)
	}

	// Check if stack is in a valid state to be terminated
	if stack.Status == entities.StatusDeploying || stack.Status == entities.StatusUpdating || stack.Status == entities.StatusTerminating {
		logger.Error("The stack is still deploying, updating or terminating, please wait for it to finish", zap.String("stackId", stackId.String()))
		return fmt.Errorf("the stack is still deploying, updating or terminating, please wait for it to finish")
	}

	go s.handleStackTermination(stackId)

	return nil
}

func (s *ThanosService) handleStackTermination(stackId uuid.UUID) {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	deploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(s.db)

	// Check if stack exists
	stack, err := stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
		return
	}

	// Update stack status to terminating
	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusTerminating); err != nil {
		logger.Error("failed to update stack status to terminating",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if updateErr := stackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToTerminate); updateErr != nil {
			logger.Error("failed to update stack status after unmarshal error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	logPath := utils.GetDestroyLogPath(stack.ID)
	thanosStack := trh_sdk_infrastructure.NewThanosStack()

	if err := thanosStack.DestroyAWSInfrastructure(&dtos.TerminateThanosRequest{
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
		if updateErr := stackRepo.UpdateStatus(stackId.String(), entities.StatusFailedToTerminate); updateErr != nil {
			logger.Error("failed to update stack status after destroy error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusTerminated); err != nil {
		logger.Error("failed to update stack status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	deployments, err := deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		logger.Error("failed to get deployments",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	for _, deployment := range deployments {
		if err := deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentStatus(entities.StatusPending)); err != nil {
			logger.Error("failed to update deployment status",
				zap.String("deploymentId", deployment.ID.String()),
				zap.String("stackId", stackId.String()),
				zap.Error(err))
			// Continue updating other deployments even if one fails
			continue
		}
	}

	logger.Info("AWS infrastructure destroyed successfully", zap.String("stackId", stackId.String()))
}

func (s *ThanosService) GetAllStacks() ([]*entities.StackEntity, error) {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	return stackRepo.GetAllStacks()
}

func (s *ThanosService) GetStackStatus(stackId uuid.UUID) (entities.Status, error) {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	return stackRepo.GetStackStatus(stackId.String())
}

func (s *ThanosService) GetStackDeployments(stackId uuid.UUID) ([]*entities.DeploymentEntity, error) {
	deploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(s.db)
	return deploymentRepo.GetDeploymentsByStackID(stackId.String())
}

func (s *ThanosService) GetStackDeploymentStatus(deploymentId uuid.UUID) (entities.DeploymentStatus, error) {
	deploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(s.db)
	return deploymentRepo.GetDeploymentStatus(deploymentId.String())
}

func (s *ThanosService) GetStackDeployment(stackId uuid.UUID, deploymentId uuid.UUID) (*entities.DeploymentEntity, error) {
	deploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(s.db)
	return deploymentRepo.GetDeploymentByID(deploymentId.String())
}

func (s *ThanosService) GetStackByID(stackId uuid.UUID) (*entities.StackEntity, error) {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	return stackRepo.GetStackByID(stackId.String())
}
