package schemas

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"gorm.io/datatypes"
)

type Stack struct {
	ID             uuid.UUID                  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Name           string                     `gorm:"column:name"`
	Status         entities.StackStatus       `gorm:"not null;column:status"`
	Reason         string                     `gorm:"column:reason"`
	Network        entities.DeploymentNetwork `gorm:"not null;column:network"`
	DeploymentPath string                     `gorm:"not null;column:deployment_path"`
	Config         datatypes.JSON             `gorm:"type:jsonb;not null;column:config"`
	Metadata       datatypes.JSON             `gorm:"type:jsonb;column:metadata"`
	CreatedAt      time.Time                  `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt      time.Time                  `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt      time.Time                  `gorm:"autoUpdateTime;column:deleted_at"`
}

func (Stack) TableName() string {
	return "stacks"
}
