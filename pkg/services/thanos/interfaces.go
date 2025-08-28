package thanos

import (
	"encoding/json"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

type DeploymentRepository interface {
	CreateDeployment(deployment *entities.DeploymentEntity) error
	GetDeploymentsByStackID(stackId string) ([]*entities.DeploymentEntity, error)
	UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	GetDeploymentByID(deploymentId string) (*entities.DeploymentEntity, error)
	GetDeploymentStatus(deploymentId string) (entities.DeploymentRunStatus, error)
	GetDeploymentsByStackIDAndStatus(stackId string, status entities.DeploymentRunStatus) ([]*entities.DeploymentEntity, error)
	UpdateStatusesByStackId(
		stackID string,
		status entities.DeploymentRunStatus,
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
	UpdateConfig(
		id string,
		config []byte,
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
	UpdateIntegrationStatusByStackID(
		stackID string,
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
		exceptTypes []string,
	) ([]*entities.IntegrationEntity, error)
	UpdateIntegrationsStatusByStackID(
		stackID string,
		status entities.DeploymentStatus,
		exceptStatuses []entities.DeploymentStatus,
		exceptTypes []string,
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

type LogRepository interface {
	CreateLog(log *entities.LogEntity) error
	GetLogsByDeploymentID(deploymentId string, limit int, afterID *string) ([]*entities.LogEntity, error)
	GetLogsByStackID(stackId string, limit int, afterID *string) ([]*entities.LogEntity, error)
}

type TaskManager interface {
	Start()
	AddTask(id string, task entities.Task)
	StopTask(id string)
	Stop()
}
