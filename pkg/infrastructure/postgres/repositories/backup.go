package repositories

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type BackupRepository struct {
	db *gorm.DB
}

func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

func (r *BackupRepository) CreateBackup(backup *entities.BackupEntity) error {
	schema := ToBackupSchema(backup)
	return r.db.Create(schema).Error
}

func (r *BackupRepository) UpdateBackup(backup *entities.BackupEntity) error {
	schema := ToBackupSchema(backup)
	return r.db.Save(schema).Error
}

func (r *BackupRepository) GetBackupByID(id uuid.UUID) (*entities.BackupEntity, error) {
	var schema schemas.Backup
	err := r.db.Where("id = ?", id).First(&schema).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("backup not found")
		}
		return nil, err
	}
	return FromBackupSchema(&schema)
}

func (r *BackupRepository) GetBackupByStackID(stackID uuid.UUID) (*entities.BackupEntity, error) {
	var schema schemas.Backup
	err := r.db.Where("stack_id = ?", stackID).First(&schema).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil if no backup found for this stack
		}
		return nil, err
	}
	return FromBackupSchema(&schema)
}

func (r *BackupRepository) GetBackupsByStackID(stackID uuid.UUID) ([]*entities.BackupEntity, error) {
	var schemas []schemas.Backup
	err := r.db.Where("stack_id = ?", stackID).Order("created_at DESC").Find(&schemas).Error
	if err != nil {
		return nil, err
	}

	backups := make([]*entities.BackupEntity, 0, len(schemas))
	for _, schema := range schemas {
		backup, err := FromBackupSchema(&schema)
		if err != nil {
			return nil, err
		}
		backups = append(backups, backup)
	}
	return backups, nil
}

func (r *BackupRepository) DeleteBackup(id uuid.UUID) error {
	return r.db.Delete(&schemas.Backup{}, id).Error
}

func (r *BackupRepository) UpsertBackup(backup *entities.BackupEntity) error {
	schema := ToBackupSchema(backup)

	// Try to find existing backup for this stack
	var existing schemas.Backup
	err := r.db.Where("stack_id = ?", backup.StackID).First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new backup
			return r.db.Create(schema).Error
		}
		return err
	}

	// Update existing backup
	schema.ID = existing.ID
	schema.CreatedAt = existing.CreatedAt
	schema.UpdatedAt = time.Now()
	return r.db.Save(schema).Error
}

// ToBackupSchema converts BackupEntity to Backup schema
func ToBackupSchema(backup *entities.BackupEntity) *schemas.Backup {
	var statusJSON datatypes.JSON
	var snapshotListJSON datatypes.JSON

	if backup.Status != nil {
		statusBytes, _ := json.Marshal(backup.Status)
		statusJSON = datatypes.JSON(statusBytes)
	}

	if backup.SnapshotList != nil {
		snapshotBytes, _ := json.Marshal(backup.SnapshotList)
		snapshotListJSON = datatypes.JSON(snapshotBytes)
	}

	schema := &schemas.Backup{
		ID:           backup.ID,
		StackID:      &backup.StackID,
		Status:       statusJSON,
		SnapshotList: snapshotListJSON,
		CreatedAt:    backup.CreatedAt,
		UpdatedAt:    backup.UpdatedAt,
	}

	if backup.DeletedAt != nil {
		schema.DeletedAt = *backup.DeletedAt
	}

	return schema
}

// FromBackupSchema converts Backup schema to BackupEntity
func FromBackupSchema(schema *schemas.Backup) (*entities.BackupEntity, error) {
	backup := &entities.BackupEntity{
		ID:        schema.ID,
		StackID:   *schema.StackID,
		CreatedAt: schema.CreatedAt,
		UpdatedAt: schema.UpdatedAt,
		DeletedAt: &schema.DeletedAt,
	}

	// Convert status JSON to BackupStatusInfo
	if len(schema.Status) > 0 {
		status, err := entities.FromJSONToBackupStatus(json.RawMessage(schema.Status))
		if err != nil {
			return nil, err
		}
		backup.Status = status
	}

	// Convert snapshot list JSON to RecoveryPoint slice
	if len(schema.SnapshotList) > 0 {
		snapshots, err := entities.FromJSONToRecoveryPoints(json.RawMessage(schema.SnapshotList))
		if err != nil {
			return nil, err
		}
		backup.SnapshotList = snapshots
	}

	return backup, nil
}
