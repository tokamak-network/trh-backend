package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"trh-backend/internal/consts"
	"trh-backend/internal/utils"
	"trh-backend/pkg/domain/entities"
	"trh-backend/pkg/domain/services"
	postgresRepositories "trh-backend/pkg/infrastructure/postgres/repositories"
	trh_sdk_infrastructure "trh-backend/pkg/infrastructure/trh_sdk"
	"trh-backend/pkg/interfaces/api/dtos"

	"github.com/google/uuid"
	trh_sdk_aws "github.com/tokamak-network/trh-sdk/pkg/cloud-provider/aws"
	trh_sdk_types "github.com/tokamak-network/trh-sdk/pkg/types"
	trh_sdk_utils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"gorm.io/gorm"
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
	fmt.Println("Deploying Thanos Stack with SDK")
	// Get the stack config
	stack, err := stackRepo.GetStack(stackId.String())
	if err != nil {
		return err
	}
	l1ContractDeployment, err := deploymentRepo.GetDeployment(l1ContractDeploymentID.String())
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
	stackConfig.LogPath = l1ContractDeployment.LogPath

	// Update the status of stack to deploying
	fmt.Println("Updating stack status to creating")
	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusCreating); err != nil {
		return err
	}

	// Channel to receive deployment status updates
	deploymentStatusChan := make(chan entities.DeploymentStatusWithID)

	// Start the deployment process in a goroutine
	fmt.Println("Starting deployment process")
	go s.deployStack(deploymentStatusChan, l1ContractDeploymentID, infrastructureDeploymentID, stackConfig)

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
	fmt.Println("Updating stack status to active")
	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusActive); err != nil {
		fmt.Println("Error updating stack status to active:", err)
		if lastError == nil {
			lastError = err
		}
	}

	return lastError
}

func (s *ThanosService) deployStack(
	statusChan chan entities.DeploymentStatusWithID,
	l1ContractDeploymentID uuid.UUID,
	infrastructureDeploymentID uuid.UUID,
	stackConfig dtos.DeployThanosRequest,
) {
	defer close(statusChan)

	thanosStack := trh_sdk_infrastructure.NewThanosStack()

	// Deploy L1 Contracts
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: l1ContractDeploymentID,
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
		LogPath:                  stackConfig.LogPath,
	}

	if err := thanosStack.DeployL1Contracts(&deployL1ContractsRequest); err != nil {
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: l1ContractDeploymentID,
			Status:       entities.DeploymentStatusFailed,
		}
		return
	}

	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: l1ContractDeploymentID,
		Status:       entities.DeploymentStatusCompleted,
	}

	// Deploy Infrastructure
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: infrastructureDeploymentID,
		Status:       entities.DeploymentStatusInProgress,
	}

	if err := thanosStack.DeployInfrastructure(); err != nil {
		statusChan <- entities.DeploymentStatusWithID{
			DeploymentID: infrastructureDeploymentID,
			Status:       entities.DeploymentStatusFailed,
		}
		return
	}

	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: infrastructureDeploymentID,
		Status:       entities.DeploymentStatusCompleted,
	}
}

func (s *ThanosService) ValidateThanosRequest(request dtos.DeployThanosRequest) error {
	if request.Network == entities.DeploymentNetworkLocalDevnet {
		return errors.New("local devnet is not supported yet")
	}

	// Validate L1 RPC URL
	if !trh_sdk_utils.IsValidL1RPC(request.L1RpcUrl) {
		fmt.Printf("invalid l1RpcUrl %s", request.L1RpcUrl)
		return errors.New("invalid l1RpcUrl")
	}

	// Validate L1 Beacon URL
	if !trh_sdk_utils.IsValidBeaconURL(request.L1BeaconUrl) {
		fmt.Printf("invalid l1BeaconUrl %s", request.L1BeaconUrl)
		return errors.New("invalid l1BeaconUrl")
	}

	// Validate AWS Access Key
	if !trh_sdk_utils.IsValidAWSAccessKey(request.AwsAccessKey) {
		fmt.Printf("invalid awsAccessKey %s", request.AwsAccessKey)
		return errors.New("invalid awsAccessKey")
	}

	// Validate AWS Secret Key
	if !trh_sdk_utils.IsValidAWSSecretKey(request.AwsSecretAccessKey) {
		fmt.Printf("invalid awsSecretKey %s", request.AwsSecretAccessKey)
		return errors.New("invalid awsSecretKey")
	}

	// Validate AWS Region
	if !trh_sdk_aws.IsAvailableRegion(request.AwsAccessKey, request.AwsSecretAccessKey, request.AwsRegion) {
		fmt.Printf("invalid awsRegion %s", request.AwsRegion)
		return errors.New("invalid awsRegion")
	}

	// Validate Chain Config
	chainID, err := utils.GetChainIDFromRPC(request.L1RpcUrl)
	if err != nil {
		fmt.Printf("invalid chainId %s", err)
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
		fmt.Printf("invalid chainConfig %s", err)
		return err
	}

	return nil
}
