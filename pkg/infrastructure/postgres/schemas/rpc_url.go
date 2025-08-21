package schemas

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RPCUrl struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Name      string         `gorm:"column:name;not null"`
	RpcUrl    string         `gorm:"column:rpc_url;not null"`
	Type      string         `gorm:"column:type;not null"`    // BeaconChain or ExecutionLayer
	Network   string         `gorm:"column:network;not null"` // Testnet or Mainnet
	CreatedAt time.Time      `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;default:null"`
}

func (RPCUrl) TableName() string {
	return "rpc_urls"
}
