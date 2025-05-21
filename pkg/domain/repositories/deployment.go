package repositories

import (
	"trh-backend/pkg/infrastructure/postgres/schemas"
)

type DeploymentRepository interface {
	CreateDeployment(deployment *schemas.Deployment) error
	UpdateDeploymentStatus(id string, status schemas.DeploymentStatus) error
}
