package repositories

import (
	"trh-backend/pkg/domain/entities"
)

type StackRepository interface {
	CreateStack(stack *entities.StackEntity) error
	GetStackByID(id string) (*entities.StackEntity, error)
	GetAllStacks() ([]*entities.StackEntity, error)
	GetStackStatus(id string) (entities.Status, error)
	UpdateStatus(id string, status entities.Status) error
	DeleteStack(id string) error
}
