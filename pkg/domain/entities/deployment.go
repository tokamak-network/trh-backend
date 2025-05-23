package entities

import (
	"github.com/google/uuid"
)

type DeploymentEntity struct {
	ID            uuid.UUID        `json:"id"`
	StackID       *uuid.UUID       `json:"stack_id"`
	IntegrationID *uuid.UUID       `json:"integration_id"`
	Step          int              `json:"step"`
	Name          string           `json:"name"`
	Status        DeploymentStatus `json:"status"`
	LogPath       string           `json:"log_path"`
}

type DeploymentStatusWithID struct {
	DeploymentID uuid.UUID
	Status       DeploymentStatus
}
