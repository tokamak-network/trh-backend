package repositories

import (
	"encoding/json"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StackPostgresRepository struct {
	db *gorm.DB
}

func NewStackRepository(db *gorm.DB) *StackPostgresRepository {
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

func (r *StackPostgresRepository) GetStackByID(
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

func (r *StackPostgresRepository) GetAllStacks() ([]*entities.StackEntity, error) {
	var stacks []schemas.Stack
	err := r.db.Find(&stacks).Error
	if err != nil {
		return nil, err
	}
	stacksEntities := make([]*entities.StackEntity, len(stacks))
	for i, stack := range stacks {
		stacksEntities[i] = &entities.StackEntity{
			ID:             stack.ID,
			Name:           stack.Name,
			Network:        stack.Network,
			Config:         json.RawMessage(stack.Config),
			DeploymentPath: stack.DeploymentPath,
			Status:         stack.Status,
		}
	}
	return stacksEntities, nil
}

func (r *StackPostgresRepository) GetStackStatus(
	id string,
) (entities.Status, error) {
	var stack schemas.Stack
	err := r.db.Where("id = ?", id).First(&stack).Error
	if err != nil {
		return entities.StatusUnknown, err
	}
	return stack.Status, nil
}
