package repositories

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/gorm"
)

type RPCUrlRepository struct {
	db *gorm.DB
}

func NewRPCUrlRepository(db *gorm.DB) *RPCUrlRepository {
	return &RPCUrlRepository{db: db}
}

func (r *RPCUrlRepository) Create(rpcUrl *entities.RPCUrlEntity) error {
	schema := &schemas.RPCUrl{
		ID:      rpcUrl.ID,
		Name:    rpcUrl.Name,
		RpcUrl:  rpcUrl.RpcUrl,
		Type:    string(rpcUrl.Type),
		Network: string(rpcUrl.Network),
	}

	result := r.db.Create(schema)
	if result.Error != nil {
		return result.Error
	}

	// Update the entity with the generated ID and timestamps
	rpcUrl.ID = schema.ID
	rpcUrl.CreatedAt = schema.CreatedAt
	rpcUrl.UpdatedAt = schema.UpdatedAt

	return nil
}

func (r *RPCUrlRepository) GetByID(id uuid.UUID) (*entities.RPCUrlEntity, error) {
	var schema schemas.RPCUrl
	result := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&schema)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("rpc url not found")
		}
		return nil, result.Error
	}

	var deletedAt *time.Time
	if !schema.DeletedAt.Time.IsZero() {
		deletedAt = &schema.DeletedAt.Time
	}

	return &entities.RPCUrlEntity{
		ID:        schema.ID,
		Name:      schema.Name,
		RpcUrl:    schema.RpcUrl,
		Type:      entities.RPCType(schema.Type),
		Network:   entities.NetworkType(schema.Network),
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
		DeletedAt: deletedAt,
	}, nil
}

func (r *RPCUrlRepository) GetByName(name string) (*entities.RPCUrlEntity, error) {
	var schema schemas.RPCUrl
	result := r.db.Where("name = ? AND deleted_at IS NULL", name).First(&schema)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("rpc url not found")
		}
		return nil, result.Error
	}

	var deletedAt *time.Time
	if !schema.DeletedAt.Time.IsZero() {
		deletedAt = &schema.DeletedAt.Time
	}

	return &entities.RPCUrlEntity{
		ID:        schema.ID,
		Name:      schema.Name,
		RpcUrl:    schema.RpcUrl,
		Type:      entities.RPCType(schema.Type),
		Network:   entities.NetworkType(schema.Network),
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
		DeletedAt: deletedAt,
	}, nil
}

func (r *RPCUrlRepository) GetAll() ([]*entities.RPCUrlEntity, error) {
	var schemas []schemas.RPCUrl
	result := r.db.Where("deleted_at IS NULL").Find(&schemas)
	if result.Error != nil {
		return nil, result.Error
	}

	rpcUrlList := make([]*entities.RPCUrlEntity, len(schemas))
	for i, schema := range schemas {
		var deletedAt *time.Time
		if !schema.DeletedAt.Time.IsZero() {
			deletedAt = &schema.DeletedAt.Time
		}

		rpcUrlList[i] = &entities.RPCUrlEntity{
			ID:        schema.ID,
			Name:      schema.Name,
			RpcUrl:    schema.RpcUrl,
			Type:      entities.RPCType(schema.Type),
			Network:   entities.NetworkType(schema.Network),
			CreatedAt: schema.CreatedAt,
			UpdatedAt: schema.UpdatedAt,
			DeletedAt: deletedAt,
		}
	}

	return rpcUrlList, nil
}

func (r *RPCUrlRepository) Update(rpcUrl *entities.RPCUrlEntity) error {
	schema := &schemas.RPCUrl{
		ID:      rpcUrl.ID,
		Name:    rpcUrl.Name,
		RpcUrl:  rpcUrl.RpcUrl,
		Type:    string(rpcUrl.Type),
		Network: string(rpcUrl.Network),
	}

	result := r.db.Model(&schemas.RPCUrl{}).Where("id = ?", rpcUrl.ID).Updates(schema)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("rpc url not found")
	}

	// Update the entity with the new timestamp
	rpcUrl.UpdatedAt = schema.UpdatedAt

	return nil
}

func (r *RPCUrlRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&schemas.RPCUrl{}, id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("rpc url not found")
	}

	return nil
}
