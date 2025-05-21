package repositories

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"trh-backend/internal/utils"
	"trh-backend/pkg/infrastructure/postgres/schemas"
	"trh-backend/pkg/interfaces/api/dtos"
)

type StackPostgresRepository struct {
	db *gorm.DB
}

func NewStackPostgresRepository(db *gorm.DB) *StackPostgresRepository {
	return &StackPostgresRepository{db: db}
}

func (r *StackPostgresRepository) CreateStack(
	stack dtos.DeployThanosRequest,
) (schemas.Stack, error) {
	deploymentId := uuid.New()
	deploymentPath := utils.GetDeploymentPath("thanos", stack.Network, deploymentId.String())
	stackJson, err := json.Marshal(stack)
	if err != nil {
		return schemas.Stack{}, err
	}
	newStack := schemas.Stack{
		ID:             deploymentId,
		Name:           "thanos",
		Status:         schemas.StatusActive,
		Network:        stack.Network,
		DeploymentPath: deploymentPath,
		Config:         datatypes.JSON(stackJson),
	}
	err = r.db.Create(&newStack).Error
	if err != nil {
		return schemas.Stack{}, err
	}
	return newStack, nil
}

func (r *StackPostgresRepository) DeleteStack(id string) error {
	return r.db.Delete(&schemas.Stack{}, id).Error
}

func (r *StackPostgresRepository) UpdateStatus(id string, status schemas.Status) error {
	return r.db.Model(&schemas.Stack{}).Where("id = ?", id).Update("status", status).Error
}
