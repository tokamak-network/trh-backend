package entities

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DeploymentEntity struct {
	ID         uuid.UUID           `json:"id"`
	StackID    *uuid.UUID          `json:"stack_id,omitempty"`
	Step       string              `json:"step"`
	Status     DeploymentRunStatus `json:"status"`
	LogPath    string              `json:"log_path"`
	Config     json.RawMessage     `json:"config"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
	StartedAt  *time.Time          `json:"started_at,omitempty"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
}

type DeploymentStatusWithID struct {
	DeploymentID uuid.UUID
	Status       DeploymentRunStatus
}
