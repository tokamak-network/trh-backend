package repositories

import (
	"trh-backend/pkg/infrastructure/postgres/schemas"
	"trh-backend/pkg/interfaces/api/dtos"
)

type StackRepository interface {
	CreateStack(stack dtos.DeployThanosRequest) (schemas.Stack, error)
	UpdateStatus(id string, status schemas.Status) error
	DeleteStack(id string) error
}
