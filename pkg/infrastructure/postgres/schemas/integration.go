package schemas

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Integration struct {
	ID        int64           `gorm:"primaryKey;autoIncrement;column:id"`
	StackID   int64           `gorm:"column:stack_id;not null;references:ID"`
	Stack     Stack           `gorm:"foreignKey:StackID"`
	Name      string          `gorm:"column:name;unique"`
	Status    Status          `gorm:"column:status"`
	Config    json.RawMessage `gorm:"column:config;type:jsonb;not null"`
	Info      json.RawMessage `gorm:"column:info;type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Integration) TableName() string {
	return "integrations"
}
