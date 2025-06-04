package entities

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type DeploymentEntity struct {
	ID             uuid.UUID
	StackID        *uuid.UUID
	IntegrationID  *uuid.UUID
	Step           int
	Status         DeploymentStatus
	LogPath        string
	DeploymentPath string
	Config         datatypes.JSON
}

type DeploymentStatusWithID struct {
	DeploymentID uuid.UUID
	Status       DeploymentStatus
}
