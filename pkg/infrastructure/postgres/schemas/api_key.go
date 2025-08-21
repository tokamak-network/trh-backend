package schemas

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ApiKey struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	ApiKey    string         `gorm:"column:api_key;not null"`
	Type      string         `gorm:"column:type;not null"`
	CreatedAt time.Time      `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;default:null"`
}

func (ApiKey) TableName() string {
	return "api_keys"
}
