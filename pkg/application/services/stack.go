package services

import (
	"trh-backend/pkg/domain/repositories"
	"trh-backend/pkg/infrastructure/postgres/schemas"
	"trh-backend/pkg/interfaces/api/dtos"
)

type StackService struct {
	StackRepository repositories.StackRepository
}

func NewStackService(stackRepository repositories.StackRepository) *StackService {
	return &StackService{StackRepository: stackRepository}
}

func (s *StackService) DeployStack(stack dtos.DeployThanosRequest) (schemas.Stack, error) {
	return s.StackRepository.CreateStack(stack)
}

func (s *StackService) DestroyStack(id string) error {
	return s.StackRepository.DeleteStack(id)
}
