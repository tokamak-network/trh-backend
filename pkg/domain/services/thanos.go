package services

import (
	"trh-backend/internal/utils"
	"trh-backend/pkg/domain/entities"

	"github.com/google/uuid"
)

type ThanosDomainService struct {
}

func NewThanosDomainService() *ThanosDomainService {
	return &ThanosDomainService{}
}

func (s *ThanosDomainService) GetThanosStackDeployments(stackId uuid.UUID) ([]entities.DeploymentEntity, error) {
	deployments := []entities.DeploymentEntity{}
	l1ContractDeploymentID := uuid.New()
	l1ContractDeployment := entities.DeploymentEntity{
		ID:            l1ContractDeploymentID,
		StackID:       &stackId,
		IntegrationID: nil,
		Step:          1,
		Name:          "thanos-l1-contract-deployment",
		Status:        entities.DeploymentStatusPending,
		LogPath:       utils.GetDeploymentLogPath(stackId, l1ContractDeploymentID),
	}
	deployments = append(deployments, l1ContractDeployment)

	thanosInfrastructureDeploymentID := uuid.New()
	thanosInfrastructureDeployment := entities.DeploymentEntity{
		ID:            thanosInfrastructureDeploymentID,
		StackID:       &stackId,
		IntegrationID: nil,
		Step:          2,
		Name:          "thanos-infrastructure-deployment",
		Status:        entities.DeploymentStatusPending,
		LogPath:       utils.GetDeploymentLogPath(stackId, thanosInfrastructureDeploymentID),
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}
