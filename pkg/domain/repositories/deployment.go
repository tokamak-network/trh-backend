package repositories

import (
	"trh-backend/pkg/domain/entities"
)

type DeploymentRepository interface {
	CreateDeployment(deployment *entities.DeploymentEntity) error
	UpdateDeploymentStatus(id string, status entities.DeploymentStatus) error
	DeleteDeployment(id string) error
}
