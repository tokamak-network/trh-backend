package entities

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Integration struct {
	ID      uuid.UUID       `json:"id"`
	StackID *uuid.UUID      `json:"stack_id"`
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Config  json.RawMessage `json:"config"`
	Info    json.RawMessage `json:"info"`
	LogPath string          `json:"log_path"`
}
