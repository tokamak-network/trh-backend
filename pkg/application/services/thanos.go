package services

import (
	"encoding/json"
	"fmt"

	"trh-backend/internal/utils"
	"trh-backend/pkg/domain/entities"
	"trh-backend/pkg/domain/services"
	postgresRepositories "trh-backend/pkg/infrastructure/postgres/repositories"
	trh_sdk "trh-backend/pkg/infrastructure/trh_sdk"
	"trh-backend/pkg/interfaces/api/dtos"

	"github.com/google/uuid"
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
	// Check if stack exists
	_, err := stackRepo.GetStack(stackId.String())
	if err != nil {
		return err
	}

	// Update the status of stack to deploying
	fmt.Println("Updating stack status to creating")
	if err := stackRepo.UpdateStatus(stackId.String(), entities.StatusCreating); err != nil {
		return err
	}

	// Channel to receive deployment status updates
	deploymentStatusChan := make(chan entities.DeploymentStatusWithID)

	// Start the deployment process in a goroutine
	fmt.Println("Starting deployment process")
	go s.deployStack(deploymentStatusChan, l1ContractDeploymentID, infrastructureDeploymentID)

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
) {
	defer close(statusChan)

	thanosStack := trh_sdk.NewThanosStack()

	// Deploy L1 Contracts
	statusChan <- entities.DeploymentStatusWithID{
		DeploymentID: l1ContractDeploymentID,
		Status:       entities.DeploymentStatusInProgress,
	}

	if err := thanosStack.DeployL1Contracts(); err != nil {
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
