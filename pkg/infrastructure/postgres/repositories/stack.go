package repositories

import (
	"encoding/json"
	"trh-backend/pkg/domain/entities"
	"trh-backend/pkg/infrastructure/postgres/schemas"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StackPostgresRepository struct {
	db *gorm.DB
}

func NewStackPostgresRepository(db *gorm.DB) *StackPostgresRepository {
	return &StackPostgresRepository{db: db}
}

func (r *StackPostgresRepository) CreateStack(
	stack *entities.StackEntity,
) error {
	newStack := schemas.Stack{
		ID:             stack.ID,
		Name:           stack.Name,
		Network:        stack.Network,
		Config:         datatypes.JSON(stack.Config),
		DeploymentPath: stack.DeploymentPath,
		Status:         stack.Status,
	}
	err := r.db.Create(&newStack).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *StackPostgresRepository) DeleteStack(
	id string,
) error {
	return r.db.Delete(&schemas.Stack{}, id).Error
}

func (r *StackPostgresRepository) UpdateStatus(
	id string,
	status entities.Status,
) error {
	return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("status", status).Error
}

func (r *StackPostgresRepository) GetStack(
	id string,
) (*entities.StackEntity, error) {
	var stack schemas.Stack
	err := r.db.Where("id = ?", id).First(&stack).Error
	if err != nil {
		return nil, err
	}
	return &entities.StackEntity{
		ID:             stack.ID,
		Name:           stack.Name,
		Network:        stack.Network,
		Config:         json.RawMessage(stack.Config),
		DeploymentPath: stack.DeploymentPath,
		Status:         stack.Status,
	}, nil
}
