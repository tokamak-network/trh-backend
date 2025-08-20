package repositories

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// JSON encoder pool to reduce memory allocations
var jsonEncoderPool = sync.Pool{
	New: func() interface{} {
		return json.NewEncoder(nil)
	},
}

type StackRepository struct {
	db *gorm.DB
}

func NewStackRepository(db *gorm.DB) *StackRepository {
	return &StackRepository{db: db}
}

func (r *StackRepository) CreateStack(
	stack *entities.StackEntity,
) error {
	newStack := ToStackEntity(stack)
	err := r.db.Create(&newStack).Error
	if err != nil {
		return err
	}
	return nil
}

func (r *StackRepository) CreateStackByTx(
	stack *entities.StackEntity,
	deployments []*entities.DeploymentEntity,
	integrations []*entities.IntegrationEntity,
) error {
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
		return err
	}

	integrationsSchema := make([]*schemas.Integration, 0)
	for _, integration := range integrations {
		integrationsSchema = append(integrationsSchema, ToIntegrationSchema(integration))
	}
	err = tx.Create(integrationsSchema).Error
	if err != nil {
		tx.Rollback()
		return err
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
	status entities.StackStatus,
	reason string,
) error {
	if reason == "" {
		return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("status", status).Error
	} else {
		return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("status", status).Update("reason", reason).Error
	}
}

func (r *StackRepository) UpdateMetadata(
	id string,
	metadata *entities.StackMetadata,
) error {
	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}
	b, err := metadata.Marshal()
	if err != nil {
		return err
	}
	return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("metadata", b).Error
}

func (r *StackRepository) GetStackByID(
	id string,
) (*entities.StackEntity, error) {
	var stack schemas.Stack
	err := r.db.Where("id = ?", id).First(&stack).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("stack with id %s not found", id)
		}
		return nil, err
	}

	metadata, err := entities.FromJSONToStackMetadata(json.RawMessage(stack.Metadata))
	if err != nil {
		return nil, err
	}

	return &entities.StackEntity{
		ID:             stack.ID,
		Name:           stack.Name,
		Type:           stack.Type,
		Network:        stack.Network,
		Config:         json.RawMessage(stack.Config),
		Metadata:       metadata,
		DeploymentPath: stack.DeploymentPath,
		Status:         stack.Status,
		CreatedAt:      stack.CreatedAt,
		UpdatedAt:      stack.UpdatedAt,
		DeletedAt:      stack.DeletedAt,
	}, nil
}

func (r *StackRepository) GetAllStacks() ([]*entities.StackEntity, error) {
	var stacks []schemas.Stack
	err := r.db.Select("id, name, type, network, config, metadata, deployment_path, status, created_at, updated_at, deleted_at").Find(&stacks).Error
	if err != nil {
		return nil, err
	}

	stacksEntities := make([]*entities.StackEntity, 0, len(stacks))
	for _, stack := range stacks {
		metadata, err := entities.FromJSONToStackMetadata(json.RawMessage(stack.Metadata))
		if err != nil {
			return nil, err
		}

		stacksEntities = append(stacksEntities, &entities.StackEntity{
			ID:             stack.ID,
			Name:           stack.Name,
			Type:           stack.Type,
			Network:        stack.Network,
			Config:         json.RawMessage(stack.Config),
			Metadata:       metadata,
			DeploymentPath: stack.DeploymentPath,
			Status:         stack.Status,
			CreatedAt:      stack.CreatedAt,
			UpdatedAt:      stack.UpdatedAt,
			DeletedAt:      stack.DeletedAt,
		})
	}
	return stacksEntities, nil
}

func (r *StackRepository) GetStackStatus(
	id string,
) (entities.StackStatus, error) {
	var stack schemas.Stack
	err := r.db.Where("id = ?", id).First(&stack).Error
	if err != nil {
		return entities.StackStatusUnknown, err
	}
	return stack.Status, nil
}

func (r *StackRepository) UpdateConfig(
	id string,
	config []byte,
) error {
	return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("config", config).Error
}

func ToStackEntity(s *entities.StackEntity) *schemas.Stack {
	return &schemas.Stack{
		ID:             s.ID,
		Name:           s.Name,
		Type:           s.Type,
		Network:        s.Network,
		Config:         datatypes.JSON(s.Config),
		DeploymentPath: s.DeploymentPath,
		Status:         s.Status,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		DeletedAt:      s.DeletedAt,
	}
}
