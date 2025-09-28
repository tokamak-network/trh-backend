package schemas

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Backup struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	StackID      *uuid.UUID     `gorm:"type:uuid;not null;column:stack_id;references:ID"`
	Stack        *Stack         `gorm:"foreignKey:StackID"`
	Status       datatypes.JSON `gorm:"type:jsonb;not null;column:status"`
	SnapshotList datatypes.JSON `gorm:"type:jsonb;not null;column:snapshot_list"`
	CreatedAt    time.Time      `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;default:null"`
}

func (Backup) TableName() string {
	return "backups"
}
