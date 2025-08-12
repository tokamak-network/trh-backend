package schemas

import (
	"time"

	"github.com/google/uuid"
)

// Log represents a single deployment log entry.
// Fields:
// - id (uuid)
// - stack_id (uuid)
// - deployment_id (uuid)
// - message (text)
// - created_at (timestamp)
type Log struct {
	ID           uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	StackID      *uuid.UUID  `gorm:"column:stack_id;not null;references:ID;index:idx_logs_stack_id"`
	Stack        *Stack      `gorm:"foreignKey:StackID"`
	DeploymentID *uuid.UUID  `gorm:"column:deployment_id;not null;references:ID;index:idx_logs_deployment_id"`
	Deployment   *Deployment `gorm:"foreignKey:DeploymentID"`
	Message      string      `gorm:"column:message;type:text;not null"`
	CreatedAt    time.Time   `gorm:"autoCreateTime;column:created_at;index:idx_logs_created_at"`
}

func (Log) TableName() string {
	return "logs"
}
