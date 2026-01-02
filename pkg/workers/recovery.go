package workers

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosSDK "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const recoveryTimeout = 5 * time.Minute

//uninstaller defines the signature for integration uninstall functions
type uninstaller func(context.Context, *thanosSDK.ThanosStack) error

// this maps integration types to their cleanup fuunctions
var uninstallers = map[enum.IntegrationType]uninstaller{
	enum.IntegrationTypeBridge:        thanos.UninstallBridge,
	enum.IntegrationTypeBlockExplorer: thanos.UninstallBlockExplorer,
	enum.IntegrationTypeMonitoring:    thanos.UninstallMonitoring,
	enum.IntegrationTypeCrossTrade:    thanos.UninstallCrossTradeBridge,
	enum.IntegrationTypeUptimeService: thanos.UninstallUptimeService,
}

//so recoverInterruptedIntegrations finds integrations stuck in inprogress state
//from server crash and cleansup their resources before marking them failed
func RecoverInterruptedIntegrations(db *gorm.DB) {
	var integrations []schemas.Integration

	err := db.Preload("Stack").
		Where("status = ?", entities.DeploymentStatusInProgress).
		Find(&integrations).Error
	if err != nil {
		logger.Error("failed to query interrupted integrations", zap.Error(err))
		return
	}

	if len(integrations) == 0 {
		return
	}

	logger.Warn("recovering interrupted integrations", zap.Int("count", len(integrations)))

	for i := range integrations {
		recoverIntegration(db, &integrations[i])
	}

	logger.Info("recovery complete")
}

func recoverIntegration(db *gorm.DB, integration *schemas.Integration) {
	id := integration.ID.String()
	typ := integration.Type

	logger.Info("recovering integration", zap.String("id", id), zap.String("type", typ))

	// tur cleanup if we have stack info
	cleanupSuccess := false
	if integration.Stack != nil {
		if err := cleanupIntegrationResources(integration); err != nil {
			logger.Warn("cleanup failed", zap.String("id", id), zap.Error(err))
		} else {
			cleanupSuccess = true
			logger.Info("cleanup successful", zap.String("id", id))
		}
	}

	if cleanupSuccess {
		// changed this to delete from db after successful cleanup instead of failed shoing on UI before
		err := db.Delete(&schemas.Integration{}, "id = ?", integration.ID).Error
		if err != nil {
			logger.Error("failed to delete integration", zap.String("id", id), zap.Error(err))
		} else {
			logger.Info("deleted interrupted integration", zap.String("id", id))
		}
	} else {
		// mark failed if cleanup didnt run or failed so user can retry manually
		err := db.Model(&schemas.Integration{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"status": entities.DeploymentStatusFailed,
				"reason": "interrupted by server restart",
			}).Error
		if err != nil {
			logger.Error("failed to update status", zap.String("id", id), zap.Error(err))
		}
	}
}

func cleanupIntegrationResources(integration *schemas.Integration) error {
	uninstall, ok := uninstallers[enum.IntegrationType(integration.Type)]
	if !ok {
		return nil
	}

	stack := integration.Stack

	// checks if deployment path exists before attempting cleanup
	if stack.DeploymentPath == "" {
		logger.Info("skipping cleanup: no deployment path")
		return nil
	}
	if _, err := os.Stat(stack.DeploymentPath); os.IsNotExist(err) {
		logger.Info("skipping cleanup: deployment path does not exist", zap.String("path", stack.DeploymentPath))
		return nil
	}

	var cfg dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
	defer cancel()

	logger.Info("attempting cleanup", zap.String("type", integration.Type), zap.String("path", stack.DeploymentPath))

	// using deployment path for log file during cleanup
	logPath := stack.DeploymentPath + "/recovery.log"

	client, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		false,
		cfg.AwsAccessKey,
		cfg.AwsSecretAccessKey,
		cfg.AwsRegion,
	)
	if err != nil {
		return err
	}

	return uninstall(ctx, client)
}
