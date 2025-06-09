package repositories

import (
	"encoding/json"
	"errors"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DeploymentRepository struct {
	db *gorm.DB
}

func NewDeploymentRepository(db *gorm.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (r *DeploymentRepository) CreateDeployment(deployment *entities.DeploymentEntity) error {
	return r.db.Create(ToDeploymentSchema(deployment)).Error
}

func (r *DeploymentRepository) UpdateDeploymentStatus(
	id string,
	status entities.DeploymentStatus,
) error {
	return r.db.Model(&schemas.Deployment{}).Where("id = ?", id).Update("status", status).Error
}

func (r *DeploymentRepository) UpdateStatusesByStackId(
	stackID string,
	status entities.DeploymentStatus,
) error {
	return r.db.Model(&schemas.Deployment{}).Where("stack_id = ?", stackID).Update("status", status).Error
}

func (r *DeploymentRepository) DeleteDeployment(id string) error {
	return r.db.Delete(&schemas.Deployment{}, id).Error
}

func (r *DeploymentRepository) GetDeploymentByID(id string) (*entities.DeploymentEntity, error) {
	var deployment schemas.Deployment
	if err := r.db.Where("id = ?", id).First(&deployment).Error; err != nil {
		return nil, err
	}
	return &entities.DeploymentEntity{
		ID:      deployment.ID,
		StackID: deployment.StackID,
		Step:    deployment.Step,
		Status:  deployment.Status,
		LogPath: deployment.LogPath,
		Config:  json.RawMessage(deployment.Config),
	}, nil
}

func (r *DeploymentRepository) GetDeploymentsByStackID(
	stackID string,
) ([]*entities.DeploymentEntity, error) {
	var deployments []schemas.Deployment
	if err := r.db.Where("stack_id = ?", stackID).Order("step asc").Find(&deployments).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No deployments found for this stack
		}
		return nil, err
	}
	deploymentsEntities := make([]*entities.DeploymentEntity, len(deployments))
	for i, deployment := range deployments {
		deploymentsEntities[i] = &entities.DeploymentEntity{
			ID:      deployment.ID,
			StackID: deployment.StackID,
			Step:    deployment.Step,
			Status:  deployment.Status,
			LogPath: deployment.LogPath,
			Config:  json.RawMessage(deployment.Config),
		}
	}
	return deploymentsEntities, nil
}

func (r *DeploymentRepository) GetDeploymentStatus(id string) (entities.DeploymentStatus, error) {
	var deployment schemas.Deployment
	if err := r.db.Where("id = ?", id).First(&deployment).Error; err != nil {
		return entities.DeploymentStatusUnknown, err
	}
	return deployment.Status, nil
}

func ToDeploymentSchema(d *entities.DeploymentEntity) *schemas.Deployment {
	return &schemas.Deployment{
		ID:      d.ID,
		StackID: d.StackID,
		Step:    d.Step,
		Status:  d.Status,
		LogPath: d.LogPath,
		Config:  datatypes.JSON(d.Config),
	}
}
