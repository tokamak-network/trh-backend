package entities

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BackupStatusInfo represents the backup status information from the SDK
type BackupStatusInfo struct {
	Status     string                 `json:"status"`
	LastBackup *time.Time             `json:"last_backup,omitempty"`
	NextBackup *time.Time             `json:"next_backup,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// RecoveryPoint represents a backup recovery point from the SDK
type RecoveryPoint struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	CreatedAt   time.Time              `json:"created_at"`
	Size        int64                  `json:"size,omitempty"`
	Status      string                 `json:"status"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BackupEntity represents a backup record in the domain layer
type BackupEntity struct {
	ID           uuid.UUID         `json:"id"`
	StackID      uuid.UUID         `json:"stack_id"`
	Status       *BackupStatusInfo `json:"status"`
	SnapshotList []RecoveryPoint   `json:"snapshot_list"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	DeletedAt    *gorm.DeletedAt   `json:"deleted_at,omitempty"`
}

// MarshalStatus marshals the backup status to JSON
func (b *BackupEntity) MarshalStatus() ([]byte, error) {
	if b.Status == nil {
		return nil, nil
	}
	return json.Marshal(b.Status)
}

// UnmarshalStatus unmarshals JSON data into backup status
func (b *BackupEntity) UnmarshalStatus(data []byte) error {
	if len(data) == 0 {
		b.Status = nil
		return nil
	}
	var status BackupStatusInfo
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}
	b.Status = &status
	return nil
}

// MarshalSnapshotList marshals the snapshot list to JSON
func (b *BackupEntity) MarshalSnapshotList() ([]byte, error) {
	return json.Marshal(b.SnapshotList)
}

// UnmarshalSnapshotList unmarshals JSON data into snapshot list
func (b *BackupEntity) UnmarshalSnapshotList(data []byte) error {
	if len(data) == 0 {
		b.SnapshotList = nil
		return nil
	}
	return json.Unmarshal(data, &b.SnapshotList)
}

// FromJSONToBackupStatus converts JSON data to BackupStatusInfo
func FromJSONToBackupStatus(data json.RawMessage) (*BackupStatusInfo, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var status BackupStatusInfo
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// FromJSONToRecoveryPoints converts JSON data to RecoveryPoint slice
func FromJSONToRecoveryPoints(data json.RawMessage) ([]RecoveryPoint, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var points []RecoveryPoint
	if err := json.Unmarshal(data, &points); err != nil {
		return nil, err
	}
	return points, nil
}
