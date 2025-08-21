package entities

import (
	"time"

	"github.com/google/uuid"
)

// RPCType represents the type of RPC URL
type RPCType string

const (
	RPCTypeBeaconChain    RPCType = "BeaconChain"
	RPCTypeExecutionLayer RPCType = "ExecutionLayer"
)

// NetworkType represents the network type
type NetworkType string

const (
	NetworkTypeTestnet NetworkType = "Testnet"
	NetworkTypeMainnet NetworkType = "Mainnet"
)

type RPCUrlEntity struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	RpcUrl    string      `json:"rpcUrl"`
	Type      RPCType     `json:"type"`
	Network   NetworkType `json:"network"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
	DeletedAt *time.Time  `json:"deletedAt,omitempty"`
}
