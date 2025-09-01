package repositories

import (
	"encoding/json"
	"errors"
	"time"

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
	now := time.Now().UTC()
	if deployment.Status == entities.DeploymentRunStatusInProgress {
		deployment.StartedAt = &now
	}
	if deployment.Status == entities.DeploymentRunStatusSuccess ||
		deployment.Status == entities.DeploymentRunStatusFailed ||
		deployment.Status == entities.DeploymentRunStatusStopped {
		deployment.FinishedAt = &now
	}
	return r.db.Create(ToDeploymentSchema(deployment)).Error
}

func (r *DeploymentRepository) UpdateDeploymentStatus(
	id string,
	status entities.DeploymentRunStatus,
) error {
	updates := map[string]interface{}{
		"status": status,
	}
	now := time.Now().UTC()
	if status == entities.DeploymentRunStatusInProgress {
		updates["started_at"] = &now
	}
	if status == entities.DeploymentRunStatusFailed || status == entities.DeploymentRunStatusSuccess {
		updates["finished_at"] = &now
	}
	return r.db.Model(&schemas.Deployment{}).
		Where("id = ?", id).
		Where("status != ?", entities.DeploymentRunStatusSuccess).
		Where("status != ?", entities.DeploymentRunStatusFailed).
		Updates(updates).Error
}

func (r *DeploymentRepository) UpdateStatusesByStackId(
	stackID string,
	status entities.DeploymentRunStatus,
) error {
	return r.db.Model(&schemas.Deployment{}).Where("stack_id = ?", stackID).
		Where("status != ?", entities.DeploymentRunStatusSuccess).
		Update("status", status).Error
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
		ID:         deployment.ID,
		StackID:    deployment.StackID,
		Step:       deployment.Step,
		Status:     deployment.Status,
		LogPath:    deployment.LogPath,
		Config:     json.RawMessage(deployment.Config),
		StartedAt:  deployment.StartedAt,
		FinishedAt: deployment.FinishedAt,
	}, nil
}

func (r *DeploymentRepository) GetDeploymentsByStackID(
	stackID string,
) ([]*entities.DeploymentEntity, error) {
	var deployments []schemas.Deployment
	if err := r.db.Where("stack_id = ?", stackID).Order("updated_at desc").Find(&deployments).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No deployments found for this stack
		}
		return nil, err
	}
	deploymentsEntities := make([]*entities.DeploymentEntity, len(deployments))
	for i, deployment := range deployments {
		deploymentsEntities[i] = &entities.DeploymentEntity{
			ID:         deployment.ID,
			StackID:    deployment.StackID,
			Step:       deployment.Step,
			Status:     deployment.Status,
			LogPath:    deployment.LogPath,
			Config:     json.RawMessage(deployment.Config),
			CreatedAt:  deployment.CreatedAt,
			UpdatedAt:  deployment.UpdatedAt,
			StartedAt:  deployment.StartedAt,
			FinishedAt: deployment.FinishedAt,
		}
	}
	return deploymentsEntities, nil
}

func (r *DeploymentRepository) GetDeploymentsByStackIDAndStatus(
	stackID string,
	status entities.DeploymentRunStatus,
) ([]*entities.DeploymentEntity, error) {
	var deployments []schemas.Deployment
	if err := r.db.Where("stack_id = ?", stackID).
		Where("status = ?", status).Order("updated_at desc").
		Find(&deployments).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No deployments found for this stack
		}
		return nil, err
	}
	deploymentsEntities := make([]*entities.DeploymentEntity, len(deployments))
	for i, deployment := range deployments {
		deploymentsEntities[i] = &entities.DeploymentEntity{
			ID:         deployment.ID,
			StackID:    deployment.StackID,
			Step:       deployment.Step,
			Status:     deployment.Status,
			LogPath:    deployment.LogPath,
			Config:     json.RawMessage(deployment.Config),
			CreatedAt:  deployment.CreatedAt,
			UpdatedAt:  deployment.UpdatedAt,
			StartedAt:  deployment.StartedAt,
			FinishedAt: deployment.FinishedAt,
		}
	}
	return deploymentsEntities, nil
}

func (r *DeploymentRepository) GetDeploymentStatus(id string) (entities.DeploymentRunStatus, error) {
	var deployment schemas.Deployment
	if err := r.db.Where("id = ?", id).First(&deployment).Error; err != nil {
		return entities.DeploymentRunStatus(""), err
	}
	return deployment.Status, nil
}

func ToDeploymentSchema(d *entities.DeploymentEntity) *schemas.Deployment {
	return &schemas.Deployment{
		ID:         d.ID,
		StackID:    d.StackID,
		Step:       d.Step,
		Status:     d.Status,
		LogPath:    d.LogPath,
		Config:     datatypes.JSON(d.Config),
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
	}
}
