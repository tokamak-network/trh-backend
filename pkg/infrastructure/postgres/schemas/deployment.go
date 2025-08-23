package schemas

import (
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Deployment struct {
	ID         uuid.UUID                    `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	StackID    *uuid.UUID                   `gorm:"column:stack_id;nullable;references:ID"`
	Stack      Stack                        `gorm:"foreignKey:StackID"`
	Step       string                       `gorm:"column:step;not null"`
	Status     entities.DeploymentRunStatus `gorm:"column:status;not null"`
	Config     datatypes.JSON               `gorm:"type:jsonb;default:null;column:config"`
	LogPath    string                       `gorm:"column:log_path"`
	StartedAt  *time.Time                   `gorm:"column:started_at;default:null"`
	FinishedAt *time.Time                   `gorm:"column:finished_at;default:null"`
	CreatedAt  time.Time                    `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt  time.Time                    `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt  gorm.DeletedAt               `gorm:"column:deleted_at;default:null"`
}

func (Deployment) TableName() string {
	return "deployments"
}
