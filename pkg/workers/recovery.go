package workers

import (
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RecoverInterruptedIntegrations marks stale inprogress integrations as failed on startup...
func RecoverInterruptedIntegrations(db *gorm.DB) {
	result := db.Model(&schemas.Integration{}).
		Where("status = ?", entities.DeploymentStatusInProgress).
		Updates(map[string]interface{}{
			"status": entities.DeploymentStatusFailed,
			"reason": "Installation interrupted by server restart",
		})

	if result.Error != nil {
		logger.Error("failed to recover interrupted integrations", zap.Error(result.Error))
		return
	}

	if result.RowsAffected > 0 {
		logger.Warn("recovered interrupted integrations", zap.Int64("count", result.RowsAffected))
	}
}
