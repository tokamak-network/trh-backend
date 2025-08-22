package dtos

import (
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

type CreateRPCUrlRequest struct {
	Name    string               `json:"name" binding:"required"`
	RpcUrl  string               `json:"rpcUrl" binding:"required"`
	Type    entities.RPCType     `json:"type" binding:"required"`
	Network entities.NetworkType `json:"network" binding:"required"`
}

func (r *CreateRPCUrlRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return ErrNameRequired
	}

	if strings.TrimSpace(r.RpcUrl) == "" {
		return ErrRpcUrlRequired
	}

	// Validate URL format
	if _, err := url.Parse(r.RpcUrl); err != nil {
		return ErrInvalidRpcUrlFormat
	}

	// Validate Type
	if r.Type != entities.RPCTypeBeaconChain && r.Type != entities.RPCTypeExecutionLayer {
		return ErrInvalidRpcType
	}

	// Validate Network
	if r.Network != entities.NetworkTypeTestnet && r.Network != entities.NetworkTypeMainnet {
		return ErrInvalidNetworkType
	}

	return nil
}

type UpdateRPCUrlRequest struct {
	Name    *string               `json:"name,omitempty"`
	RpcUrl  *string               `json:"rpcUrl,omitempty"`
	Type    *entities.RPCType     `json:"type,omitempty"`
	Network *entities.NetworkType `json:"network,omitempty"`
}

func (r *UpdateRPCUrlRequest) Validate() error {
	// At least one field must be provided
	if r.Name == nil && r.RpcUrl == nil && r.Type == nil && r.Network == nil {
		return ErrNoFieldsToUpdate
	}

	// Validate name if provided
	if r.Name != nil {
		if strings.TrimSpace(*r.Name) == "" {
			return ErrNameRequired
		}
	}

	// Validate RPC URL if provided
	if r.RpcUrl != nil {
		if strings.TrimSpace(*r.RpcUrl) == "" {
			return ErrRpcUrlRequired
		}
		// Validate URL format
		if _, err := url.Parse(*r.RpcUrl); err != nil {
			return ErrInvalidRpcUrlFormat
		}
	}

	// Validate Type if provided
	if r.Type != nil {
		if *r.Type != entities.RPCTypeBeaconChain && *r.Type != entities.RPCTypeExecutionLayer {
			return ErrInvalidRpcType
		}
	}

	// Validate Network if provided
	if r.Network != nil {
		if *r.Network != entities.NetworkTypeTestnet && *r.Network != entities.NetworkTypeMainnet {
			return ErrInvalidNetworkType
		}
	}

	return nil
}

type RPCUrlResponse struct {
	ID        uuid.UUID            `json:"id"`
	Name      string               `json:"name"`
	RpcUrl    string               `json:"rpcUrl"`
	Type      entities.RPCType     `json:"type"`
	Network   entities.NetworkType `json:"network"`
	CreatedAt string               `json:"createdAt"`
	UpdatedAt string               `json:"updatedAt"`
	DeletedAt *string              `json:"deletedAt,omitempty"`
}

type RPCUrlListResponse struct {
	RpcUrls []RPCUrlResponse `json:"rpcUrls"`
	Total   int              `json:"total"`
}

type RPCUrlCreateResponse struct {
	RpcUrl RPCUrlResponse `json:"rpcUrl"`
}

type RPCUrlUpdateResponse struct {
	RpcUrl RPCUrlResponse `json:"rpcUrl"`
}

// Custom errors for RPC URL
var (
	ErrRpcUrlRequired      = &ValidationError{Message: "RPC URL is required"}
	ErrInvalidRpcUrlFormat = &ValidationError{Message: "invalid RPC URL format"}
	ErrInvalidRpcType      = &ValidationError{Message: "invalid RPC type, must be BeaconChain or ExecutionLayer"}
	ErrInvalidNetworkType  = &ValidationError{Message: "invalid network type, must be Testnet or Mainnet"}
	ErrRpcUrlNotFound      = &ValidationError{Message: "RPC URL not found"}
	ErrRpcUrlNameExists    = &ValidationError{Message: "RPC URL with this name already exists"}
)
