package schemas

import (
	"time"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Deployment struct {
	ID            uuid.UUID                 `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	StackID       *uuid.UUID                `gorm:"column:stack_id;nullable;references:ID"`
	Stack         Stack                     `gorm:"foreignKey:StackID"`
	IntegrationID *uuid.UUID                `gorm:"column:integration_id;nullable;references:ID"`
	Integration   Integration               `gorm:"foreignKey:IntegrationID"`
	Step          int                       `gorm:"column:step;not null"`
	Name          string                    `gorm:"column:name"`
	Status        entities.DeploymentStatus `gorm:"column:status;not null"`
	Config        datatypes.JSON            `gorm:"type:jsonb;not null;column:config"`
	LogPath       string                    `gorm:"column:log_path"`
	CreatedAt     time.Time                 `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt     time.Time                 `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt     time.Time                 `gorm:"autoUpdateTime;column:deleted_at"`
}

func (Deployment) TableName() string {
	return "deployments"
}
