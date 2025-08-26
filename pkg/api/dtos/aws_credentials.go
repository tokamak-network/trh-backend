package dtos

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type CreateAWSCredentialsRequest struct {
	Name            string `json:"name" binding:"required"`
	AccessKeyID     string `json:"accessKeyId" binding:"required"`
	SecretAccessKey string `json:"secretAccessKey" binding:"required"`
}

func (r *CreateAWSCredentialsRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return ErrNameRequired
	}

	if strings.TrimSpace(r.AccessKeyID) == "" {
		return ErrAccessKeyIDRequired
	}

	if strings.TrimSpace(r.SecretAccessKey) == "" {
		return ErrSecretAccessKeyRequired
	}

	// Validate Access Key ID format (typically starts with AKIA and is 20 characters)
	accessKeyRegex := regexp.MustCompile(`^AKIA[0-9A-Z]{16}$`)
	if !accessKeyRegex.MatchString(r.AccessKeyID) {
		return ErrInvalidAccessKeyID
	}

	// Validate Secret Access Key format (40 characters, alphanumeric)
	secretKeyRegex := regexp.MustCompile(`^[A-Za-z0-9/+=]{40}$`)
	if !secretKeyRegex.MatchString(r.SecretAccessKey) {
		return ErrInvalidSecretAccessKey
	}

	return nil
}

type UpdateAWSCredentialsRequest struct {
	Name            *string `json:"name,omitempty"`
	AccessKeyID     *string `json:"accessKeyId,omitempty"`
	SecretAccessKey *string `json:"secretAccessKey,omitempty"`
}

func (r *UpdateAWSCredentialsRequest) Validate() error {
	// At least one field must be provided
	if r.Name == nil && r.AccessKeyID == nil && r.SecretAccessKey == nil {
		return ErrNoFieldsToUpdate
	}

	// Validate name if provided
	if r.Name != nil {
		if strings.TrimSpace(*r.Name) == "" {
			return ErrNameRequired
		}
	}

	// Validate access key ID if provided
	if r.AccessKeyID != nil {
		if strings.TrimSpace(*r.AccessKeyID) == "" {
			return ErrAccessKeyIDRequired
		}
		// Validate Access Key ID format (typically starts with AKIA and is 20 characters)
		accessKeyRegex := regexp.MustCompile(`^AKIA[0-9A-Z]{16}$`)
		if !accessKeyRegex.MatchString(*r.AccessKeyID) {
			return ErrInvalidAccessKeyID
		}
	}

	// Validate secret access key if provided
	if r.SecretAccessKey != nil {
		if strings.TrimSpace(*r.SecretAccessKey) == "" {
			return ErrSecretAccessKeyRequired
		}
		// Validate Secret Access Key format (40 characters, alphanumeric)
		secretKeyRegex := regexp.MustCompile(`^[A-Za-z0-9/+=]{40}$`)
		if !secretKeyRegex.MatchString(*r.SecretAccessKey) {
			return ErrInvalidSecretAccessKey
		}
	}

	return nil
}

type AWSCredentialsResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	AccessKeyID     string    `json:"accessKeyId"`
	SecretAccessKey string    `json:"secretAccessKey"`
	CreatedAt       string    `json:"createdAt"`
	UpdatedAt       string    `json:"updatedAt"`
	DeletedAt       *string   `json:"deletedAt,omitempty"`
}

type AWSCredentialsListResponse struct {
	Credentials []AWSCredentialsResponse `json:"credentials"`
	Total       int                      `json:"total"`
}

type AWSCredentialsCreateResponse struct {
	Credential AWSCredentialsResponse `json:"credential"`
}

type AWSCredentialsUpdateResponse struct {
	Credential AWSCredentialsResponse `json:"credential"`
}

type GetAvailableRegionsRequest struct {
	AccessKeyID     string `json:"accessKeyId" binding:"required"`
	SecretAccessKey string `json:"secretAccessKey" binding:"required"`
}

type GetAvailableRegionsResponse struct {
	Regions []string `json:"regions"`
	Total   int      `json:"total"`
}

// Custom errors
var (
	ErrNameRequired            = &ValidationError{Message: "name is required"}
	ErrAccessKeyIDRequired     = &ValidationError{Message: "access key ID is required"}
	ErrSecretAccessKeyRequired = &ValidationError{Message: "secret access key is required"}
	ErrInvalidAccessKeyID      = &ValidationError{Message: "invalid access key ID format"}
	ErrInvalidSecretAccessKey  = &ValidationError{Message: "invalid secret access key format"}
	ErrAWSCredentialsNotFound  = &ValidationError{Message: "aws credentials not found"}
	ErrNameAlreadyExists       = &ValidationError{Message: "aws credentials with this name already exists"}
	ErrNoFieldsToUpdate        = &ValidationError{Message: "at least one field must be provided for update"}
)
