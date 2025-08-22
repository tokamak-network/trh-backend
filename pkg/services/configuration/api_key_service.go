package configuration

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
)

type ApiKeyService struct {
	repo *repositories.ApiKeyRepository
}

func NewApiKeyService(repo *repositories.ApiKeyRepository) *ApiKeyService {
	return &ApiKeyService{
		repo: repo,
	}
}

func (s *ApiKeyService) Create(req *dtos.CreateApiKeyRequest) (*dtos.ApiKeyCreateResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Create entity
	apiKey := &entities.ApiKeyEntity{
		ID:     uuid.New(),
		ApiKey: req.ApiKey,
		Type:   req.Type,
	}

	// Save to database
	if err := s.repo.Create(apiKey); err != nil {
		return nil, err
	}

	// Convert to response
	return &dtos.ApiKeyCreateResponse{
		ApiKey: *s.entityToResponse(apiKey),
	}, nil
}

func (s *ApiKeyService) GetByID(id uuid.UUID) (*dtos.ApiKeyResponse, error) {
	apiKey, err := s.repo.GetByID(id)
	if err != nil {
		return nil, dtos.ErrApiKeyNotFound
	}

	return s.entityToResponse(apiKey), nil
}

func (s *ApiKeyService) GetAll() (*dtos.ApiKeyListResponse, error) {
	apiKeys, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	responses := make([]dtos.ApiKeyResponse, len(apiKeys))
	for i, apiKey := range apiKeys {
		responses[i] = *s.entityToResponse(apiKey)
	}

	return &dtos.ApiKeyListResponse{
		ApiKeys: responses,
		Total:   len(responses),
	}, nil
}

func (s *ApiKeyService) Update(id uuid.UUID, req *dtos.UpdateApiKeyRequest) (*dtos.ApiKeyUpdateResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check if API Key exists
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, dtos.ErrApiKeyNotFound
	}

	// Update entity with only provided fields
	if req.ApiKey != nil {
		existing.ApiKey = *req.ApiKey
	}
	if req.Type != nil {
		existing.Type = *req.Type
	}

	// Save to database
	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}

	return &dtos.ApiKeyUpdateResponse{
		ApiKey: *s.entityToResponse(existing),
	}, nil
}

func (s *ApiKeyService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

func (s *ApiKeyService) entityToResponse(entity *entities.ApiKeyEntity) *dtos.ApiKeyResponse {
	response := &dtos.ApiKeyResponse{
		ID:        entity.ID,
		ApiKey:    entity.ApiKey,
		Type:      entity.Type,
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}

	if entity.DeletedAt != nil {
		deletedAt := entity.DeletedAt.Format(time.RFC3339)
		response.DeletedAt = &deletedAt
	}

	return response
}
