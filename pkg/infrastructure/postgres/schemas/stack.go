package schemas

import (
	"time"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Stack struct {
	ID             uuid.UUID                  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Name           string                     `gorm:"column:name"`
	Status         entities.Status            `gorm:"not null;column:status"`
	Reason         string                     `gorm:"column:reason"`
	Network        entities.DeploymentNetwork `gorm:"not null;column:network"`
	DeploymentPath string                     `gorm:"not null;column:deployment_path"`
	Config         datatypes.JSON             `gorm:"type:jsonb;not null;column:config"`
	Info           datatypes.JSON             `gorm:"type:jsonb;column:info"`
	CreatedAt      time.Time                  `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt      time.Time                  `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt      gorm.DeletedAt             `gorm:"index;column:deleted_at"`
}

func (Stack) TableName() string {
	return "stacks"
}
