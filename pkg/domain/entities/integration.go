package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type IntegrationInfo []byte

func (info IntegrationInfo) ToJson() (json.RawMessage, error) {
	if info == nil {
		return nil, nil // Return nil if no info is provided
	}
	return json.RawMessage(info), nil
}

type IntegrationEntity struct {
	ID      uuid.UUID       `json:"id"`
	StackID *uuid.UUID      `json:"stack_id"`
	Type    string          `json:"type"`
	Status  string          `json:"status"`
	Config  json.RawMessage `json:"config"`
	Info    json.RawMessage `json:"info"`
	LogPath string          `json:"log_path"`
	Reason  string          `json:"reason"`
}
