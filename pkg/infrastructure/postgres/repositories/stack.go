package repositories

import (
	"encoding/json"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StackRepository struct {
	db *gorm.DB
}

func NewStackRepository(db *gorm.DB) *StackRepository {
	return &StackRepository{db: db}
}

func (r *StackRepository) CreateStack(
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

func (r *StackRepository) CreateStackByTx(stack *entities.StackEntity, deployments []*entities.DeploymentEntity) error {
	tx := r.db.Begin()
	err := tx.Create(ToStackEntity(stack)).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	deploymentsSchema := make([]*schemas.Deployment, 0)
	for _, deployment := range deployments {
		deploymentsSchema = append(deploymentsSchema, ToDeploymentSchema(deployment))
	}
	err = tx.Create(deploymentsSchema).Error
	if err != nil {
		tx.Rollback()
	}

	return tx.Commit().Error
}

func (r *StackRepository) DeleteStack(
	id string,
) error {
	return r.db.Delete(&schemas.Stack{}, id).Error
}

func (r *StackRepository) UpdateStatus(
	id string,
	status entities.Status,
	reason string,
) error {
	if reason == "" {
		return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("status", status).Error
	} else {
		return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("status", status).Update("reason", reason).Error
	}
}

func (r *StackRepository) GetStackByID(
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

func (r *StackRepository) GetAllStacks() ([]*entities.StackEntity, error) {
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

func (r *StackRepository) GetStackStatus(
	id string,
) (entities.Status, error) {
	var stack schemas.Stack
	err := r.db.Where("id = ?", id).First(&stack).Error
	if err != nil {
		return entities.StatusUnknown, err
	}
	return stack.Status, nil
}

func ToStackEntity(s *entities.StackEntity) *schemas.Stack {
	return &schemas.Stack{
		ID:             s.ID,
		Name:           s.Name,
		Network:        s.Network,
		Config:         datatypes.JSON(s.Config),
		DeploymentPath: s.DeploymentPath,
		Status:         s.Status,
	}
}
