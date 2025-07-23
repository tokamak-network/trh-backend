package entities

import (
	"time"

	"github.com/google/uuid"
)

type AWSCredentialsEntity struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	AccessKeyID     string     `json:"accessKeyId"`
	SecretAccessKey string     `json:"secretAccessKey"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeletedAt       *time.Time `json:"deletedAt,omitempty"`
}
