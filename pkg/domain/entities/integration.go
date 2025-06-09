package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type IntegrationInfo struct {
	Url string `json:"url"`
}

func (info *IntegrationInfo) ToJson() (json.RawMessage, error) {
	if info == nil {
		return nil, nil // Return nil if no info is provided
	}
	data, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type IntegrationEntity struct {
	ID      uuid.UUID       `json:"id"`
	StackID *uuid.UUID      `json:"stack_id"`
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Config  json.RawMessage `json:"config"`
	Info    json.RawMessage `json:"info"`
	LogPath string          `json:"log_path"`
}
