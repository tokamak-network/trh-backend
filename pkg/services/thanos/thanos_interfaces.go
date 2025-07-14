package thanos

import (
	"encoding/json"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
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
		integrations []*entities.IntegrationEntity,
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
	UpdateIntegrationStatusWithReason(
		id string,
		status entities.DeploymentStatus,
		reason string,
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
		metadata entities.IntegrationInfo,
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
