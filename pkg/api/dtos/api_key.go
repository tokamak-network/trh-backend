package dtos

import (
	"strings"

	"github.com/google/uuid"
)

type CreateApiKeyRequest struct {
	ApiKey string `json:"apiKey" binding:"required"`
	Type   string `json:"type" binding:"required"`
}

func (r *CreateApiKeyRequest) Validate() error {
	if strings.TrimSpace(r.ApiKey) == "" {
		return ErrApiKeyRequired
	}

	if strings.TrimSpace(r.Type) == "" {
		return ErrApiKeyTypeRequired
	}

	return nil
}

type UpdateApiKeyRequest struct {
	ApiKey *string `json:"apiKey,omitempty"`
	Type   *string `json:"type,omitempty"`
}

func (r *UpdateApiKeyRequest) Validate() error {
	// At least one field must be provided
	if r.ApiKey == nil && r.Type == nil {
		return ErrNoFieldsToUpdate
	}

	// Validate API key if provided
	if r.ApiKey != nil {
		if strings.TrimSpace(*r.ApiKey) == "" {
			return ErrApiKeyRequired
		}
	}

	// Validate type if provided
	if r.Type != nil {
		if strings.TrimSpace(*r.Type) == "" {
			return ErrApiKeyTypeRequired
		}
	}

	return nil
}

type ApiKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	ApiKey    string    `json:"apiKey"`
	Type      string    `json:"type"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
	DeletedAt *string   `json:"deletedAt,omitempty"`
}

type ApiKeyListResponse struct {
	ApiKeys []ApiKeyResponse `json:"apiKeys"`
	Total   int              `json:"total"`
}

type ApiKeyCreateResponse struct {
	ApiKey ApiKeyResponse `json:"apiKey"`
}

type ApiKeyUpdateResponse struct {
	ApiKey ApiKeyResponse `json:"apiKey"`
}

// Custom errors for API Key
var (
	ErrApiKeyRequired     = &ValidationError{Message: "API key is required"}
	ErrApiKeyTypeRequired = &ValidationError{Message: "API key type is required"}
	ErrApiKeyNotFound     = &ValidationError{Message: "API key not found"}
)
