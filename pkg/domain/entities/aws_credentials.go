package entities

import (
	"time"

	"github.com/google/uuid"
)

type AWSCredentialsEntity struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	AccessKeyID     string     `json:"access_key_id"`
	SecretAccessKey string     `json:"secret_access_key"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}
