package services

import (
	"encoding/json"

	"trh-backend/internal/utils"
	"trh-backend/pkg/domain/entities"
	"trh-backend/pkg/domain/services"
	postgresRepositories "trh-backend/pkg/infrastructure/postgres/repositories"
	"trh-backend/pkg/interfaces/api/dtos"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ThanosService struct {
	db                  *gorm.DB
	ThanosDomainService *services.ThanosDomainService
}

func NewThanosService(db *gorm.DB, thanosDomainService *services.ThanosDomainService) *ThanosService {
	return &ThanosService{db: db, ThanosDomainService: thanosDomainService}
}

func (s *ThanosService) DeployThanosStack(request dtos.DeployThanosRequest) (uuid.UUID, error) {
	stackId := uuid.New()
	stackName := "thanos"
	deploymentPath := utils.GetDeploymentPath(stackName, request.Network, stackId.String())
	config, err := json.Marshal(request)
	if err != nil {
		return uuid.Nil, err
	}
	stack := entities.StackEntity{
		ID:             stackId,
		Name:           stackName,
		Network:        request.Network,
		Config:         config,
		DeploymentPath: deploymentPath,
		Status:         entities.StatusActive,
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return uuid.Nil, tx.Error
	}
	stackRepo := postgresRepositories.NewStackPostgresRepository(tx)
	err = stackRepo.CreateStack(&stack)
	if err != nil {
		tx.Rollback()
		return uuid.Nil, err
	}

	deployments, err := s.ThanosDomainService.GetThanosStackDeployments(stackId)

	deploymentRepo := postgresRepositories.NewDeploymentPostgresRepository(tx)
	if err != nil {
		return uuid.Nil, err
	}

	for _, deployment := range deployments {
		err = deploymentRepo.CreateDeployment(&deployment)
		if err != nil {
			tx.Rollback()
			return uuid.Nil, err
		}
	}

	err = tx.Commit().Error
	if err != nil {
		return uuid.Nil, err
	}

	return stackId, nil
}
func (s *ThanosService) DestroyThanosStack(id string) error {
	stackRepo := postgresRepositories.NewStackPostgresRepository(s.db)
	return stackRepo.DeleteStack(id)
}
