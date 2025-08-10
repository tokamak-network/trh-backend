package entities

import (
	"time"

	"github.com/google/uuid"
)

// LogEntity represents a single deployment log entry
type LogEntity struct {
	ID           uuid.UUID  `json:"id"`
	StackID      *uuid.UUID `json:"stack_id,omitempty"`
	DeploymentID *uuid.UUID `json:"deployment_id,omitempty"`
	Message      string     `json:"message"`
	CreatedAt    time.Time  `json:"created_at"`
}
