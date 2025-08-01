package schemas

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AWSCredentials struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Name            string         `gorm:"column:name;not null"`
	AccessKeyID     string         `gorm:"column:access_key_id;not null"`
	SecretAccessKey string         `gorm:"column:secret_access_key;not null"`
	CreatedAt       time.Time      `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;default:null"`
}

func (AWSCredentials) TableName() string {
	return "aws_credentials"
}
