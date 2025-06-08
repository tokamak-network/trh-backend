package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type DeploymentEntity struct {
	ID             uuid.UUID
	StackID        *uuid.UUID
	Step           int
	Status         DeploymentStatus
	LogPath        string
	DeploymentPath string
	Config         json.RawMessage
}

type DeploymentStatusWithID struct {
	DeploymentID uuid.UUID
	Status       DeploymentStatus
}
