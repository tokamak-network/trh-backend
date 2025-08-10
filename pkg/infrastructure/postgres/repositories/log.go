package repositories

import (
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"gorm.io/gorm"
)

type LogRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{db: db}
}

func (r *LogRepository) CreateLog(log *entities.LogEntity) error {
	model := &schemas.Log{
		StackID:      log.StackID,
		DeploymentID: log.DeploymentID,
		Message:      log.Message,
	}
	return r.db.Create(model).Error
}

func (r *LogRepository) GetLogsByDeploymentID(deploymentId string, limit int, afterID *string) ([]*entities.LogEntity, error) {
	var models []schemas.Log

	// If afterID provided, use it as a cursor based on (created_at, id)
	if afterID != nil && *afterID != "" {
		// fetch cursor
		var cursor schemas.Log
		if err := r.db.First(&cursor, "id = ?", *afterID).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return nil, err
			}
			// if not found, treat like no cursor
			afterID = nil
		} else {
			// Query logs strictly after the cursor
			if err := r.db.Where("deployment_id = ? AND (created_at > ?) OR (deployment_id = ? AND created_at = ? AND id > ?)",
				deploymentId, cursor.CreatedAt,
				deploymentId, cursor.CreatedAt, cursor.ID,
			).Order("created_at ASC, id ASC").Limit(limit).Find(&models).Error; err != nil {
				return nil, err
			}
		}
	}

	if afterID == nil || *afterID == "" {
		// No cursor: return the last N rows by created_at DESC then reverse to ascending
		if err := r.db.Where("deployment_id = ?", deploymentId).
			Order("created_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
			return nil, err
		}
		// reverse slice to ascending order
		for i, j := 0, len(models)-1; i < j; i, j = i+1, j-1 {
			models[i], models[j] = models[j], models[i]
		}
	}

	logs := make([]*entities.LogEntity, 0, len(models))
	for i := range models {
		m := models[i]
		logs = append(logs, &entities.LogEntity{
			ID:           m.ID,
			StackID:      m.StackID,
			DeploymentID: m.DeploymentID,
			Message:      m.Message,
			CreatedAt:    m.CreatedAt,
		})
	}
	return logs, nil
}

func (r *LogRepository) GetLogsByStackID(stackId string, limit int, afterID *string) ([]*entities.LogEntity, error) {
	var models []schemas.Log

	if afterID != nil && *afterID != "" {
		var cursor schemas.Log
		if err := r.db.First(&cursor, "id = ?", *afterID).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return nil, err
			}
			afterID = nil
		} else {
			if err := r.db.Where("stack_id = ? AND (created_at > ?) OR (stack_id = ? AND created_at = ? AND id > ?)",
				stackId, cursor.CreatedAt,
				stackId, cursor.CreatedAt, cursor.ID,
			).Order("created_at ASC, id ASC").Limit(limit).Find(&models).Error; err != nil {
				return nil, err
			}
		}
	}

	if afterID == nil || *afterID == "" {
		if err := r.db.Where("stack_id = ?", stackId).
			Order("created_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
			return nil, err
		}
		for i, j := 0, len(models)-1; i < j; i, j = i+1, j-1 {
			models[i], models[j] = models[j], models[i]
		}
	}

	logs := make([]*entities.LogEntity, 0, len(models))
	for i := range models {
		m := models[i]
		logs = append(logs, &entities.LogEntity{
			ID:           m.ID,
			StackID:      m.StackID,
			DeploymentID: m.DeploymentID,
			Message:      m.Message,
			CreatedAt:    m.CreatedAt,
		})
	}
	return logs, nil
}
