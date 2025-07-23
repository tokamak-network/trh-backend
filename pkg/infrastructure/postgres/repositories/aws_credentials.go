package repositories

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/gorm"
)

type AWSCredentialsRepository struct {
	db *gorm.DB
}

func NewAWSCredentialsRepository(db *gorm.DB) *AWSCredentialsRepository {
	return &AWSCredentialsRepository{db: db}
}

func (r *AWSCredentialsRepository) Create(credentials *entities.AWSCredentialsEntity) error {
	schema := &schemas.AWSCredentials{
		ID:              credentials.ID,
		Name:            credentials.Name,
		AccessKeyID:     credentials.AccessKeyID,
		SecretAccessKey: credentials.SecretAccessKey,
	}

	result := r.db.Create(schema)
	if result.Error != nil {
		return result.Error
	}

	// Update the entity with the generated ID and timestamps
	credentials.ID = schema.ID
	credentials.CreatedAt = schema.CreatedAt
	credentials.UpdatedAt = schema.UpdatedAt

	return nil
}

func (r *AWSCredentialsRepository) GetByID(id uuid.UUID) (*entities.AWSCredentialsEntity, error) {
	var schema schemas.AWSCredentials
	result := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&schema)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("aws credentials not found")
		}
		return nil, result.Error
	}

	var deletedAt *time.Time
	if !schema.DeletedAt.Time.IsZero() {
		deletedAt = &schema.DeletedAt.Time
	}

	return &entities.AWSCredentialsEntity{
		ID:              schema.ID,
		Name:            schema.Name,
		AccessKeyID:     schema.AccessKeyID,
		SecretAccessKey: schema.SecretAccessKey,
		CreatedAt:       schema.CreatedAt,
		UpdatedAt:       schema.UpdatedAt,
		DeletedAt:       deletedAt,
	}, nil
}

func (r *AWSCredentialsRepository) GetByName(name string) (*entities.AWSCredentialsEntity, error) {
	var schema schemas.AWSCredentials
	result := r.db.Where("name = ? AND deleted_at IS NULL", name).First(&schema)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("aws credentials not found")
		}
		return nil, result.Error
	}

	var deletedAt *time.Time
	if !schema.DeletedAt.Time.IsZero() {
		deletedAt = &schema.DeletedAt.Time
	}

	return &entities.AWSCredentialsEntity{
		ID:              schema.ID,
		Name:            schema.Name,
		AccessKeyID:     schema.AccessKeyID,
		SecretAccessKey: schema.SecretAccessKey,
		CreatedAt:       schema.CreatedAt,
		UpdatedAt:       schema.UpdatedAt,
		DeletedAt:       deletedAt,
	}, nil
}

func (r *AWSCredentialsRepository) GetAll() ([]*entities.AWSCredentialsEntity, error) {
	var schemas []schemas.AWSCredentials
	result := r.db.Where("deleted_at IS NULL").Find(&schemas)
	if result.Error != nil {
		return nil, result.Error
	}

	credentialsList := make([]*entities.AWSCredentialsEntity, len(schemas))
	for i, schema := range schemas {
		var deletedAt *time.Time
		if !schema.DeletedAt.Time.IsZero() {
			deletedAt = &schema.DeletedAt.Time
		}

		credentialsList[i] = &entities.AWSCredentialsEntity{
			ID:              schema.ID,
			Name:            schema.Name,
			AccessKeyID:     schema.AccessKeyID,
			SecretAccessKey: schema.SecretAccessKey,
			CreatedAt:       schema.CreatedAt,
			UpdatedAt:       schema.UpdatedAt,
			DeletedAt:       deletedAt,
		}
	}

	return credentialsList, nil
}

func (r *AWSCredentialsRepository) Update(credentials *entities.AWSCredentialsEntity) error {
	schema := &schemas.AWSCredentials{
		ID:              credentials.ID,
		Name:            credentials.Name,
		AccessKeyID:     credentials.AccessKeyID,
		SecretAccessKey: credentials.SecretAccessKey,
	}

	result := r.db.Model(&schemas.AWSCredentials{}).Where("id = ?", credentials.ID).Updates(schema)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("aws credentials not found")
	}

	// Update the entity with the new timestamp
	credentials.UpdatedAt = schema.UpdatedAt

	return nil
}

func (r *AWSCredentialsRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&schemas.AWSCredentials{}, id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("aws credentials not found")
	}

	return nil
}
