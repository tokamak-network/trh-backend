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

func (s *ThanosService) DeployThanosStack(request dtos.DeployThanosRequest) (uuid.UUID, error) {
	stackId := uuid.New()
	stackName := "thanos"
	deploymentPath := utils.GetDeploymentPath(stackName, request.Network, stackId.String())
	config, err := json.Marshal(request)
	if err != nil {
		return uuid.Nil, err
	}
	stack := entities.StackEntity{
		ID:             stackId,
		Name:           stackName,
		Network:        request.Network,
		Config:         config,
		DeploymentPath: deploymentPath,
		Status:         entities.StatusActive,
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return uuid.Nil, tx.Error
	}
	stackRepo := postgresRepositories.NewStackPostgresRepository(tx)
	err = stackRepo.CreateStack(&stack)
	if err != nil {
		tx.Rollback()
		return uuid.Nil, err
	}

	deployments, err := s.ThanosDomainService.GetThanosStackDeployments(stackId)

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

	// Create new repository instances with the main db connection
	mainStackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	mainDeploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(s.db)

	go s.DeployThanosStackWithSDK(stackId, deployments[0].ID, deployments[1].ID, mainStackRepo, mainDeploymentRepo)

	return stackId, nil
}
func (s *ThanosService) DestroyThanosStack(id string) error {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	return stackRepo.DeleteStack(id)
}

func (s *ThanosService) DeployThanosStackWithSDK(
	stackId uuid.UUID,
	l1ContractDeploymentID uuid.UUID,
	infrastructureDeploymentID uuid.UUID,
	stackRepo *postgresRepositories.StackPostgresRepository,
	deploymentRepo *postgresRepositories.DeploymentPostgresRepository,
) error {
	logger.Info("Deploying Thanos Stack with SDK")
	// Get the stack config
	stack, err := stackRepo.GetStack(stackId.String())
	if err != nil {
		return err
	}

	l1ContractDeployment, err := deploymentRepo.GetDeployment(l1ContractDeploymentID.String())
	if err != nil {
		return err
	}

	infrastructureDeployment, err := deploymentRepo.GetDeployment(infrastructureDeploymentID.String())
	if err != nil {
		return err
	}

	stackConfig := dtos.DeployThanosRequest{}
	deploymentPath := stack.DeploymentPath
	err = json.Unmarshal(stack.Config, &stackConfig)
	if err != nil {
		return err
	}
	stackConfig.DeploymentPath = deploymentPath

	// Update the status of stack to deploying
	logger.Info("Updating stack status to creating")
	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusCreating); err != nil {
		return err
	}

	// Channel to receive deployment status updates
	deploymentStatusChan := make(chan entities.DeploymentStatusWithID)

	// Start the deployment process in a goroutine
	logger.Info("Starting deployment process")
	go s.deployStack(deploymentStatusChan, l1ContractDeployment, infrastructureDeployment, stackConfig)

	// Process deployment status updates
	var lastError error
	for status := range deploymentStatusChan {
		if err := deploymentRepo.UpdateDeploymentStatus(status.DeploymentID.String(), status.Status); err != nil {
			lastError = err
		}
		if status.Status == entities.DeploymentStatusFailed {
			lastError = fmt.Errorf("deployment %s failed", status.DeploymentID)
		}
	}

	// Update stack status to active regardless of deployment outcome
	logger.Info("Updating stack status to active")
	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusActive); err != nil {
		logger.Error("Error updating stack status to active:", zap.Error(err))
		if lastError == nil {
			lastError = err
		}
	}

	return lastError
}

func (s *ThanosService) deployStack(
	statusChan chan entities.DeploymentStatusWithID,
	l1ContractDeployment *entities.DeploymentEntity,
	infrastructureDeployment *entities.DeploymentEntity,
	stackConfig dtos.DeployThanosRequest,
) {
	defer close(statusChan)

	thanosStack := trh_sdk_infrastructure.NewThanosStack()

	// Deploy L1 Contracts
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: l1ContractDeployment.ID,
		Status:       entities.DeploymentStatusInProgress,
	}

	deployL1ContractsRequest := dtos.DeployL1ContractsRequest{
		Network:                  stackConfig.Network,
		L1RpcUrl:                 stackConfig.L1RpcUrl,
		L2BlockTime:              stackConfig.L2BlockTime,
		BatchSubmissionFrequency: stackConfig.BatchSubmissionFrequency,
		OutputRootFrequency:      stackConfig.OutputRootFrequency,
		ChallengePeriod:          stackConfig.ChallengePeriod,
		AdminAccount:             stackConfig.AdminAccount,
		SequencerAccount:         stackConfig.SequencerAccount,
		BatcherAccount:           stackConfig.BatcherAccount,
		ProposerAccount:          stackConfig.ProposerAccount,
		DeploymentPath:           stackConfig.DeploymentPath,
		LogPath:                  l1ContractDeployment.LogPath,
	}

	if err := thanosStack.DeployL1Contracts(&deployL1ContractsRequest); err != nil {
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: l1ContractDeployment.ID,
			Status:       entities.DeploymentStatusFailed,
		}
		return
	}

	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: l1ContractDeployment.ID,
		Status:       entities.DeploymentStatusCompleted,
	}

	// Deploy Infrastructure
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: infrastructureDeployment.ID,
		Status:       entities.DeploymentStatusInProgress,
	}

	deployThanosAWSInfraRequest := dtos.DeployThanosAWSInfraRequest{
		ChainName:          stackConfig.ChainName,
		Network:            string(stackConfig.Network),
		L1BeaconUrl:        stackConfig.L1BeaconUrl,
		AwsAccessKey:       stackConfig.AwsAccessKey,
		AwsSecretAccessKey: stackConfig.AwsSecretAccessKey,
		AwsRegion:          stackConfig.AwsRegion,
		DeploymentPath:     stackConfig.DeploymentPath,
		LogPath:            infrastructureDeployment.LogPath,
	}

	if err := thanosStack.DeployAWSInfrastructure(&deployThanosAWSInfraRequest); err != nil {
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: infrastructureDeployment.ID,
			Status:       entities.DeploymentStatusFailed,
		}
		return
	}

	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: infrastructureDeployment.ID,
		Status:       entities.DeploymentStatusCompleted,
	}
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
