package repositories

import (
	"trh-backend/pkg/domain/entities"
	"trh-backend/pkg/infrastructure/postgres/schemas"

	"gorm.io/gorm"
)

type DeploymentPostgresRepository struct {
	db *gorm.DB
}

func NewDeploymentPostgresRepository(db *gorm.DB) *DeploymentPostgresRepository {
	return &DeploymentPostgresRepository{db: db}
}

func (r *DeploymentPostgresRepository) CreateDeployment(deployment *entities.DeploymentEntity) error {
	newDeployment := schemas.Deployment{
		ID:            deployment.ID,
		StackID:       deployment.StackID,
		IntegrationID: deployment.IntegrationID,
		Step:          deployment.Step,
		Name:          deployment.Name,
		Status:        deployment.Status,
		LogPath:       deployment.LogPath,
	}
	return r.db.Create(&newDeployment).Error
}

func (r *DeploymentPostgresRepository) UpdateDeploymentStatus(id string, status entities.DeploymentStatus) error {
	return r.db.Model(&schemas.Deployment{}).Where("id = ?", id).Update("status", status).Error
}

func (r *DeploymentPostgresRepository) DeleteDeployment(id string) error {
	return r.db.Delete(&schemas.Deployment{}, id).Error
}

func (r *DeploymentPostgresRepository) GetDeployment(id string) (*entities.DeploymentEntity, error) {
	var deployment schemas.Deployment
	if err := r.db.Where("id = ?", id).First(&deployment).Error; err != nil {
		return nil, err
	}
	return &entities.DeploymentEntity{ID: deployment.ID, StackID: deployment.StackID, IntegrationID: deployment.IntegrationID, Step: deployment.Step, Name: deployment.Name, Status: deployment.Status, LogPath: deployment.LogPath}, nil
}
