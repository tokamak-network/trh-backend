package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type StackEntity struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Network        DeploymentNetwork `json:"network"`
	Config         json.RawMessage   `json:"config"`
	DeploymentPath string            `json:"deployment_path"`
	Metadata       json.RawMessage   `json:"metadata"`
	Status         StackStatus       `json:"status"`
}
