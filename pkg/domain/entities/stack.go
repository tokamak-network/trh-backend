package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type StackMetadata struct {
	L2Url            string `json:"l2_url"`
	BridgeUrl        string `json:"bridge_url,omitempty"`
	BlockExplorerUrl string `json:"block_explorer_url,omitempty"`
}

func (m *StackMetadata) Marshal() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func FromJSONToStackMetadata(data json.RawMessage) (*StackMetadata, error) {
	var metadata StackMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

type StackEntity struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Network        DeploymentNetwork `json:"network"`
	Config         json.RawMessage   `json:"config"`
	DeploymentPath string            `json:"deployment_path"`
	Metadata       *StackMetadata    `json:"metadata"`
	Status         StackStatus       `json:"status"`
}
