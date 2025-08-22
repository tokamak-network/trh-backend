package repositories

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/gorm"
)

type ApiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository(db *gorm.DB) *ApiKeyRepository {
	return &ApiKeyRepository{db: db}
}

func (r *ApiKeyRepository) Create(apiKey *entities.ApiKeyEntity) error {
	schema := &schemas.ApiKey{
		ID:     apiKey.ID,
		ApiKey: apiKey.ApiKey,
		Type:   apiKey.Type,
	}

	result := r.db.Create(schema)
	if result.Error != nil {
		return result.Error
	}

	// Update the entity with the generated ID and timestamps
	apiKey.ID = schema.ID
	apiKey.CreatedAt = schema.CreatedAt
	apiKey.UpdatedAt = schema.UpdatedAt

	return nil
}

func (r *ApiKeyRepository) GetByID(id uuid.UUID) (*entities.ApiKeyEntity, error) {
	var schema schemas.ApiKey
	result := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&schema)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("api key not found")
		}
		return nil, result.Error
	}

	var deletedAt *time.Time
	if !schema.DeletedAt.Time.IsZero() {
		deletedAt = &schema.DeletedAt.Time
	}

	return &entities.ApiKeyEntity{
		ID:        schema.ID,
		ApiKey:    schema.ApiKey,
		Type:      schema.Type,
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
		DeletedAt: deletedAt,
	}, nil
}

func (r *ApiKeyRepository) GetAll() ([]*entities.ApiKeyEntity, error) {
	var schemas []schemas.ApiKey
	result := r.db.Where("deleted_at IS NULL").Find(&schemas)
	if result.Error != nil {
		return nil, result.Error
	}

	apiKeyList := make([]*entities.ApiKeyEntity, len(schemas))
	for i, schema := range schemas {
		var deletedAt *time.Time
		if !schema.DeletedAt.Time.IsZero() {
			deletedAt = &schema.DeletedAt.Time
		}

		apiKeyList[i] = &entities.ApiKeyEntity{
			ID:        schema.ID,
			ApiKey:    schema.ApiKey,
			Type:      schema.Type,
			CreatedAt: schema.CreatedAt,
			UpdatedAt: schema.UpdatedAt,
			DeletedAt: deletedAt,
		}
	}

	return apiKeyList, nil
}

func (r *ApiKeyRepository) Update(apiKey *entities.ApiKeyEntity) error {
	schema := &schemas.ApiKey{
		ID:     apiKey.ID,
		ApiKey: apiKey.ApiKey,
		Type:   apiKey.Type,
	}

	result := r.db.Model(&schemas.ApiKey{}).Where("id = ?", apiKey.ID).Updates(schema)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("api key not found")
	}

	// Update the entity with the new timestamp
	apiKey.UpdatedAt = schema.UpdatedAt

	return nil
}

func (r *ApiKeyRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&schemas.ApiKey{}, id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("api key not found")
	}

	return nil
}
