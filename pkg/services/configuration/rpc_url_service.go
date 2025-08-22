package configuration

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
)

type RPCUrlService struct {
	repo *repositories.RPCUrlRepository
}

func NewRPCUrlService(repo *repositories.RPCUrlRepository) *RPCUrlService {
	return &RPCUrlService{
		repo: repo,
	}
}

func (s *RPCUrlService) Create(req *dtos.CreateRPCUrlRequest) (*dtos.RPCUrlCreateResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check if name already exists
	_, err := s.repo.GetByName(req.Name)
	if err == nil {
		return nil, dtos.ErrRpcUrlNameExists
	}
	// If error is not "not found", it's a real error
	if err.Error() != "rpc url not found" {
		return nil, err
	}

	// Create entity
	rpcUrl := &entities.RPCUrlEntity{
		ID:      uuid.New(),
		Name:    req.Name,
		RpcUrl:  req.RpcUrl,
		Type:    req.Type,
		Network: req.Network,
	}

	// Save to database
	if err := s.repo.Create(rpcUrl); err != nil {
		return nil, err
	}

	// Convert to response
	return &dtos.RPCUrlCreateResponse{
		RpcUrl: *s.entityToResponse(rpcUrl),
	}, nil
}

func (s *RPCUrlService) GetByID(id uuid.UUID) (*dtos.RPCUrlResponse, error) {
	rpcUrl, err := s.repo.GetByID(id)
	if err != nil {
		return nil, dtos.ErrRpcUrlNotFound
	}

	return s.entityToResponse(rpcUrl), nil
}

func (s *RPCUrlService) GetAll() (*dtos.RPCUrlListResponse, error) {
	rpcUrls, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	responses := make([]dtos.RPCUrlResponse, len(rpcUrls))
	for i, rpcUrl := range rpcUrls {
		responses[i] = *s.entityToResponse(rpcUrl)
	}

	return &dtos.RPCUrlListResponse{
		RpcUrls: responses,
		Total:   len(responses),
	}, nil
}

func (s *RPCUrlService) Update(id uuid.UUID, req *dtos.UpdateRPCUrlRequest) (*dtos.RPCUrlUpdateResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check if RPC URL exists
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, dtos.ErrRpcUrlNotFound
	}

	// Check if new name conflicts with existing RPC URLs (excluding current one)
	if req.Name != nil && *req.Name != existing.Name {
		_, err := s.repo.GetByName(*req.Name)
		if err == nil {
			return nil, dtos.ErrRpcUrlNameExists
		}
		// If error is not "not found", it's a real error
		if err.Error() != "rpc url not found" {
			return nil, err
		}
	}

	// Update entity with only provided fields
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.RpcUrl != nil {
		existing.RpcUrl = *req.RpcUrl
	}
	if req.Type != nil {
		existing.Type = *req.Type
	}
	if req.Network != nil {
		existing.Network = *req.Network
	}

	// Save to database
	if err := s.repo.Update(existing); err != nil {
		return nil, err
	}

	return &dtos.RPCUrlUpdateResponse{
		RpcUrl: *s.entityToResponse(existing),
	}, nil
}

func (s *RPCUrlService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

func (s *RPCUrlService) entityToResponse(entity *entities.RPCUrlEntity) *dtos.RPCUrlResponse {
	response := &dtos.RPCUrlResponse{
		ID:        entity.ID,
		Name:      entity.Name,
		RpcUrl:    entity.RpcUrl,
		Type:      entity.Type,
		Network:   entity.Network,
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}

	if entity.DeletedAt != nil {
		deletedAt := entity.DeletedAt.Format(time.RFC3339)
		response.DeletedAt = &deletedAt
	}

	return response
}
