package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type DeploymentEntity struct {
	ID      uuid.UUID        `json:"id"`
	StackID *uuid.UUID       `json:"stack_id,omitempty"`
	Step    string           `json:"step"`
	Status  DeploymentStatus `json:"status"`
	LogPath string           `json:"log_path"`
	Config  json.RawMessage  `json:"config"`
}

type DeploymentStatusWithID struct {
	DeploymentID uuid.UUID
	Status       DeploymentStatus
}
