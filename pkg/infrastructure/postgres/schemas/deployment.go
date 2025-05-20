package schemas

import (
	"time"

	"gorm.io/gorm"
)

type Deployment struct {
	ID            int64            `gorm:"primaryKey;autoIncrement;column:id"`
	StackID       int64            `gorm:"column:stack_id;not null;references:ID"`
	Stack         Stack            `gorm:"foreignKey:StackID"`
	IntegrationID int64            `gorm:"column:integration_id;not null;references:ID"`
	Integration   Integration      `gorm:"foreignKey:IntegrationID"`
	Step          int              `gorm:"column:step;not null"`
	Name          string           `gorm:"column:name"`
	Status        DeploymentStatus `gorm:"column:status;not null"`
	LogPath       string           `gorm:"column:log_path"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (Deployment) TableName() string {
	return "deployments"
}
