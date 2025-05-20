package schemas

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Stack struct {
	ID             int64             `gorm:"primaryKey;autoIncrement;column:id"`
	Name           string            `gorm:"unique;column:name"`
	Status         Status            `gorm:"not null;column:status"`
	Network        DeploymentNetwork `gorm:"not null;column:network"`
	DeploymentPath string            `gorm:"not null;column:deployment_path"`
	Config         json.RawMessage   `gorm:"type:jsonb;not null;column:config"`
	Info           json.RawMessage   `gorm:"type:jsonb;column:info"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (Stack) TableName() string {
	return "stacks"
}
