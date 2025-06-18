package entities

type Response struct {
	Status  uint64 `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
