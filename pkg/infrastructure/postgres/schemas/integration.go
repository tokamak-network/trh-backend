package schemas

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Integration struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	StackID   *uuid.UUID      `gorm:"column:stack_id;not null;references:ID"`
	Stack     *Stack          `gorm:"foreignKey:StackID"`
	Name      string          `gorm:"column:name;not null"`
	LogPath   string          `gorm:"column:log_path"`
	Status    entities.Status `gorm:"column:status;not null"`
	Config    datatypes.JSON  `gorm:"column:config;type:jsonb;not null"`
	Info      datatypes.JSON  `gorm:"column:info;type:jsonb"`
	CreatedAt time.Time       `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time       `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt gorm.DeletedAt  `gorm:"autoUpdateTime;column:deleted_at"`
}

func (Integration) TableName() string {
	return "integrations"
}
