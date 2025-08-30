package repositories

import (
	"encoding/json"
	"errors"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type IntegrationRepository struct {
	db *gorm.DB
}

func NewIntegrationRepository(db *gorm.DB) *IntegrationRepository {
	return &IntegrationRepository{db: db}
}

func (r *IntegrationRepository) CreateIntegration(
	integration *entities.IntegrationEntity,
) error {
	newIntegration := ToIntegrationSchema(integration)
	err := r.db.Create(newIntegration).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *IntegrationRepository) UpdateIntegrationStatus(
	id string,
	status entities.DeploymentStatus,
) error {
	return r.db.Model(&schemas.Integration{}).Where("id = ?", id).Update("status", status).Error
}

func (r *IntegrationRepository) UpdateIntegrationStatusByStackID(
	stackID string,
	status entities.DeploymentStatus,
) error {
	return r.db.Model(&schemas.Integration{}).Where("stack_id = ?", stackID).
		Where("status != ?", entities.DeploymentStatusCompleted).
		Update("status", status).Error
}

func (r *IntegrationRepository) UpdateIntegrationStatusWithReason(
	id string,
	status entities.DeploymentStatus,
	reason string,
) error {
	return r.db.Model(&schemas.Integration{}).Where("id = ?", id).Update("status", status).Update("reason", reason).Error
}

func (r *IntegrationRepository) UpdateMetadataAfterInstalled(
	id string,
	metadata entities.IntegrationInfo,
) error {
	if metadata == nil {
		return r.db.Model(&schemas.Integration{}).
			Where("id = ?", id).
			Update("status", entities.DeploymentStatusCompleted).
			Error
	}
	return r.db.Model(&schemas.Integration{}).
		Where("id = ?", id).
		Update("info", metadata).
		Update("status", entities.DeploymentStatusCompleted).
		Error
}

func (r *IntegrationRepository) UpdateConfig(
	id string,
	config json.RawMessage,
) error {
	if config == nil {
		return nil // No metadata to update
	}
	return r.db.Model(&schemas.Integration{}).
		Where("id = ?", id).
		Update("config", config).
		Error
}

func (r *IntegrationRepository) UpdateIntegrationsStatusByStackID(
	stackID string,
	status entities.DeploymentStatus,
	exceptStatuses []entities.DeploymentStatus,
	exceptTypes []string,
) error {
	query := r.db.Model(&schemas.Integration{}).Where("stack_id = ?", stackID)
	if len(exceptStatuses) > 0 {
		query = query.Where("status NOT IN (?)", exceptStatuses)
	}
	if len(exceptTypes) > 0 {
		query = query.Where("type NOT IN (?)", exceptTypes)
	}
	return query.Update("status", status).Error
}

func (r *IntegrationRepository) GetInstalledIntegration(
	stackId string,
	integrationType string,
) (*entities.IntegrationEntity, error) {
	var integration schemas.Integration
	if err := r.db.Where("stack_id = ?", stackId).Where("type", integrationType).Where("status IN (?)", []string{
		string(entities.DeploymentStatusCompleted),
		string(entities.DeploymentStatusFailed),
		string(entities.DeploymentStatusInProgress),
		string(entities.DeploymentStatusPending),
		string(entities.DeploymentStatusUnknown),
	}).First(&integration).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No integration found
		}
		return nil, err
	}
	return ToIntegrationEntity(&integration), nil
}

func (r *IntegrationRepository) GetActiveIntegrations(
	stackId string,
	integrationType string,
) ([]*entities.IntegrationEntity, error) {
	var integrations []schemas.Integration
	if err := r.db.Where("stack_id = ?", stackId).Where("type = ?", integrationType).Where("status != ?", entities.DeploymentStatusTerminated).Order("created_at desc").Find(&integrations).Error; err != nil {
		return nil, err
	}
	integrationEntities := make([]*entities.IntegrationEntity, len(integrations))
	for i, integration := range integrations {
		integrationEntities[i] = ToIntegrationEntity(&integration)
	}
	return integrationEntities, nil
}

func (r *IntegrationRepository) GetIntegration(
	stackId string,
	integrationType string,
) (*entities.IntegrationEntity, error) {
	var integration schemas.Integration
	if err := r.db.Where("stack_id = ?", stackId).Where("type", integrationType).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No integration found
		}
		return nil, err
	}
	return ToIntegrationEntity(&integration), nil
}

func (r *IntegrationRepository) GetIntegrationById(
	id string,
) (*entities.IntegrationEntity, error) {
	var integration schemas.Integration
	if err := r.db.Where("id = ?", id).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No integration found
		}
		return nil, err
	}
	return ToIntegrationEntity(&integration), nil
}

func (r *IntegrationRepository) GetIntegrationsByStackID(
	stackID string,
) ([]*entities.IntegrationEntity, error) {
	var integrations []schemas.Integration
	if err := r.db.Where("stack_id = ?", stackID).Order("created_at asc").Find(&integrations).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No integrations found for this stack
		}
		return nil, err
	}
	integrationEntities := make([]*entities.IntegrationEntity, len(integrations))
	for i, integration := range integrations {
		integrationEntities[i] = ToIntegrationEntity(&integration)
	}
	return integrationEntities, nil
}

func (r *IntegrationRepository) GetActiveIntegrationsByStackID(
	stackId string,
	exceptTypes []string,
) ([]*entities.IntegrationEntity, error) {
	var integrations []schemas.Integration
	query := r.db.Where("stack_id = ?", stackId).Where("status != ?", entities.DeploymentStatusTerminated).Order("created_at asc")
	if len(exceptTypes) > 0 {
		query = query.Where("type NOT IN (?)", exceptTypes)
	}
	if err := query.Find(&integrations).Error; err != nil {
		return nil, err
	}
	integrationEntities := make([]*entities.IntegrationEntity, len(integrations))
	for i, integration := range integrations {
		integrationEntities[i] = ToIntegrationEntity(&integration)
	}
	return integrationEntities, nil
}

func ToIntegrationSchema(
	integration *entities.IntegrationEntity,
) *schemas.Integration {
	return &schemas.Integration{
		ID:      integration.ID,
		StackID: integration.StackID,
		Type:    integration.Type,
		Status:  entities.DeploymentStatus(integration.Status),
		Config:  datatypes.JSON(integration.Config),
		Info:    datatypes.JSON(integration.Info),
		LogPath: integration.LogPath,
	}
}

func ToIntegrationEntity(
	integration *schemas.Integration,
) *entities.IntegrationEntity {
	return &entities.IntegrationEntity{
		ID:      integration.ID,
		StackID: integration.StackID,
		Type:    integration.Type,
		Status:  string(integration.Status),
		Config:  json.RawMessage(integration.Config),
		Info:    json.RawMessage(integration.Info),
		LogPath: integration.LogPath,
	}
}
