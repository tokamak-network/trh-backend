package repositories

import (
	"encoding/json"

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
	integration *entities.Integration,
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

func (r *IntegrationRepository) UpdateIntegrationsStatusByStackID(
	stackID string,
	status entities.DeploymentStatus,
) error {
	return r.db.Model(&schemas.Integration{}).Where("stack_id = ?", stackID).Update("status", status).Error
}

func (r *IntegrationRepository) GetIntegration(
	stackId string,
	name string,
) (*entities.Integration, error) {
	var integration schemas.Integration
	if err := r.db.Where("stack_id = ?", stackId).Where("name", name).Where("status = ?", entities.DeploymentStatusCompleted).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No integration found
		}
		return nil, err
	}
	return ToIntegrationEntity(&integration), nil
}

func (r *IntegrationRepository) GetIntegrationsByStackID(
	stackID string,
) ([]*entities.Integration, error) {
	var integrations []schemas.Integration
	if err := r.db.Where("stack_id = ?", stackID).Order("created_at asc").Find(&integrations).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No integrations found for this stack
		}
		return nil, err
	}
	integrationEntities := make([]*entities.Integration, len(integrations))
	for i, integration := range integrations {
		integrationEntities[i] = ToIntegrationEntity(&integration)
	}
	return integrationEntities, nil
}

func ToIntegrationSchema(
	integration *entities.Integration,
) *schemas.Integration {
	return &schemas.Integration{
		ID:      integration.ID,
		StackID: integration.StackID,
		Name:    integration.Name,
		Status:  entities.DeploymentStatus(integration.Status),
		Config:  datatypes.JSON(integration.Config),
		Info:    datatypes.JSON(integration.Info),
		LogPath: integration.LogPath,
	}
}

func ToIntegrationEntity(
	integration *schemas.Integration,
) *entities.Integration {
	return &entities.Integration{
		ID:      integration.ID,
		StackID: integration.StackID,
		Name:    integration.Name,
		Status:  string(integration.Status),
		Config:  json.RawMessage(integration.Config),
		Info:    json.RawMessage(integration.Info),
		LogPath: integration.LogPath,
	}
}
