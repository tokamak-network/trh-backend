package repositories

import (
	"trh-backend/pkg/domain/entities"
)

type DeploymentRepository interface {
	CreateDeployment(deployment *entities.DeploymentEntity) error
	GetDeploymentByID(id string) (*entities.DeploymentEntity, error)
	GetDeploymentsByStackID(stackID string) ([]*entities.DeploymentEntity, error)
	UpdateDeploymentStatus(id string, status entities.DeploymentStatus) error
	GetDeploymentStatus(id string) (entities.DeploymentStatus, error)
	DeleteDeployment(id string) error
}
