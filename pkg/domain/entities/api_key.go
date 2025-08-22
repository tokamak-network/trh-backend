package entities

import (
	"time"

	"github.com/google/uuid"
)

type ApiKeyEntity struct {
	ID        uuid.UUID  `json:"id"`
	ApiKey    string     `json:"apiKey"`
	Type      string     `json:"type"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}
