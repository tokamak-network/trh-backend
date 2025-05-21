package repositories

import (
	"trh-backend/pkg/infrastructure/postgres/schemas"

	"gorm.io/gorm"
)

type DeploymentRepository struct {
	db *gorm.DB
}

func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (r *DeploymentRepository) CreateDeployment(deployment *schemas.Deployment) error {
	return r.db.Create(deployment).Error
}

func (r *DeploymentRepository) UpdateDeploymentStatus(id string, status schemas.DeploymentStatus) error {
	return r.db.Model(&schemas.Deployment{}).Where("id = ?", id).Update("status", status).Error
}
