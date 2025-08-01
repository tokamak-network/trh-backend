package schemas

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Integration struct {
	ID        uuid.UUID                 `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	StackID   *uuid.UUID                `gorm:"column:stack_id;not null;references:ID"`
	Stack     *Stack                    `gorm:"foreignKey:StackID"`
	Type      string                    `gorm:"column:type;not null"`
	LogPath   string                    `gorm:"column:log_path"`
	Status    entities.DeploymentStatus `gorm:"column:status;not null"`
	Config    datatypes.JSON            `gorm:"column:config;type:jsonb;default:null"`
	Info      datatypes.JSON            `gorm:"column:info;type:jsonb;default:null"`
	Reason    string                    `gorm:"column:reason;default:null"`
	CreatedAt time.Time                 `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time                 `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt gorm.DeletedAt            `gorm:"column:deleted_at;default:null"`
}

func (Integration) TableName() string {
	return "integrations"
}
