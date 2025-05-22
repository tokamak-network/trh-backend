package repositories

import (
	"trh-backend/pkg/domain/entities"
)

type StackRepository interface {
	CreateStack(stack *entities.StackEntity) error
	UpdateStatus(id string, status entities.Status) error
	DeleteStack(id string) error
}
