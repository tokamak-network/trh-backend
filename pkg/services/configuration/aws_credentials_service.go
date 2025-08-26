package configuration

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-sdk/pkg/cloud-provider/aws"
)

type AWSCredentialsService struct {
	repo *repositories.AWSCredentialsRepository
}

func NewAWSCredentialsService(repo *repositories.AWSCredentialsRepository) *AWSCredentialsService {
	return &AWSCredentialsService{
		repo: repo,
	}
}

func (s *AWSCredentialsService) Create(req *dtos.CreateAWSCredentialsRequest) (*dtos.AWSCredentialsCreateResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check if name already exists
	_, err := s.repo.GetByName(req.Name)
	if err == nil {
		return nil, dtos.ErrNameAlreadyExists
	}
	// If error is not "not found", it's a real error
	if err.Error() != "aws credentials not found" {
		return nil, err
	}

	// Create entity
	credentials := &entities.AWSCredentialsEntity{
		ID:              uuid.New(),
		Name:            req.Name,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
	}

	// Save to database
	if err := s.repo.Create(credentials); err != nil {
		return nil, err
	}

	// Convert to response
	return &dtos.AWSCredentialsCreateResponse{
		Credential: *s.entityToResponse(credentials),
	}, nil
}

func (s *AWSCredentialsService) GetByID(id uuid.UUID) (*dtos.AWSCredentialsResponse, error) {
	credentials, err := s.repo.GetByID(id)
	if err != nil {
		return nil, dtos.ErrAWSCredentialsNotFound
	}

	return s.entityToResponse(credentials), nil
}

func (s *AWSCredentialsService) GetAll() (*dtos.AWSCredentialsListResponse, error) {
	credentials, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	responses := make([]dtos.AWSCredentialsResponse, len(credentials))
	for i, cred := range credentials {
		responses[i] = *s.entityToResponse(cred)
	}

	return &dtos.AWSCredentialsListResponse{
		Credentials: responses,
		Total:       len(responses),
	}, nil
}

func (s *AWSCredentialsService) Update(id uuid.UUID, req *dtos.UpdateAWSCredentialsRequest) (*dtos.AWSCredentialsUpdateResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check if credentials exist
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, dtos.ErrAWSCredentialsNotFound
	}

	// Check if new name conflicts with existing credentials (excluding current one)
	if req.Name != nil && *req.Name != existing.Name {
		_, err := s.repo.GetByName(*req.Name)
		if err == nil {
			return nil, dtos.ErrNameAlreadyExists
		}
		// If error is not "not found", it's a real error
		if err.Error() != "aws credentials not found" {
			return nil, err
		}
	}

	// Update entity with only provided fields
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.AccessKeyID != nil {
		existing.AccessKeyID = *req.AccessKeyID
	}
	if req.SecretAccessKey != nil {
		existing.SecretAccessKey = *req.SecretAccessKey
	}

	// Save to database
	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}

	return &dtos.AWSCredentialsUpdateResponse{
		Credential: *s.entityToResponse(existing),
	}, nil
}

func (s *AWSCredentialsService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

func (s *AWSCredentialsService) entityToResponse(entity *entities.AWSCredentialsEntity) *dtos.AWSCredentialsResponse {
	response := &dtos.AWSCredentialsResponse{
		ID:              entity.ID,
		Name:            entity.Name,
		AccessKeyID:     entity.AccessKeyID,
		SecretAccessKey: entity.SecretAccessKey,
		CreatedAt:       entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       entity.UpdatedAt.Format(time.RFC3339),
	}

	if entity.DeletedAt != nil {
		deletedAt := entity.DeletedAt.Format(time.RFC3339)
		response.DeletedAt = &deletedAt
	}

	return response
}

func (s *AWSCredentialsService) GetAvailableRegions(req *dtos.GetAvailableRegionsRequest) (*dtos.GetAvailableRegionsResponse, error) {
	regions, err := aws.GetAvailableRegions(req.AccessKeyID, req.SecretAccessKey, "us-east-1") // aws default bootstrap region
	if err != nil {
		return &dtos.GetAvailableRegionsResponse{
			Regions: []string{},
			Total:   0,
		}, nil
	}

	return &dtos.GetAvailableRegionsResponse{
		Regions: regions,
		Total:   len(regions),
	}, nil
}
