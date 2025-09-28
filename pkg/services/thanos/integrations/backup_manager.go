package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosSDK "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	"go.uber.org/zap"
)

type BackupManager struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	}
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	}

	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
	}

	backupRepo interface {
		UpsertBackup(backup *entities.BackupEntity) error
		GetBackupByStackID(stackID uuid.UUID) (*entities.BackupEntity, error)
	}

	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	}
}

func NewBackupManager(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	},
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	},
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
	},
	backupRepo interface {
		UpsertBackup(backup *entities.BackupEntity) error
		GetBackupByStackID(stackID uuid.UUID) (*entities.BackupEntity, error)
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *BackupManager {
	return &BackupManager{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		backupRepo:      backupRepo,
		taskManager:     taskManager,
	}
}

func (b *BackupManager) GetBackupStatus(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "backup-status")
	thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
	if err != nil {
		logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	backupStatus, err := thanos.GetBackupStatus(ctx, thanosSDK)
	if err != nil {
		logger.Error("failed to get backup status", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    backupStatus,
	}, nil
}

func (b *BackupManager) RefreshBackupData(ctx context.Context, stackId uuid.UUID) error {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
		return err
	}

	if stack.Status != entities.StackStatusDeployed {
		logger.Warn("stack is not deployed, skipping backup data refresh", zap.String("stackId", stackId.String()))
		return nil
	}

	// Add background task to refresh backup data
	b.taskManager.AddTask(fmt.Sprintf("refresh-backup-data-%s", stackId), func(ctx context.Context) {
		logger.Info("starting backup data refresh", zap.String("stackId", stackId.String()))

		// Get Thanos client
		logPath := utils.GetLogPath(stack.ID, "refresh-backup-data")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		// Initialize backup entity
		backupEntity := &entities.BackupEntity{
			StackID:   stackId,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Get existing backup if it exists
		existingBackup, err := b.backupRepo.GetBackupByStackID(stackId)
		if err != nil {
			logger.Error("failed to get existing backup", zap.String("stackId", stackId.String()), zap.Error(err))
		} else if existingBackup != nil {
			backupEntity.ID = existingBackup.ID
			backupEntity.CreatedAt = existingBackup.CreatedAt
		}

		// Get backup status in background
		backupStatus, err := thanos.GetBackupStatus(ctx, thanosSDK)
		if err != nil {
			logger.Error("failed to get backup status", zap.String("stackId", stackId.String()), zap.Error(err))
		} else {
			// Convert SDK backup status to entity format
			statusInfo := &entities.BackupStatusInfo{
				Status:  "unknown",
				Details: make(map[string]interface{}),
			}

			// Map the SDK response to our status info structure
			if backupStatus != nil {
				statusBytes, err := json.Marshal(backupStatus)
				if err == nil {
					var statusMap map[string]interface{}
					if json.Unmarshal(statusBytes, &statusMap) == nil {
						statusInfo.Details = statusMap
						if status, ok := statusMap["status"].(string); ok {
							statusInfo.Status = status
						}
					}
				}
			}

			backupEntity.Status = statusInfo
		}

		// Get 20 snapshot entry points in background
		backupCheckpoints, err := thanos.GetListBackup(ctx, thanosSDK, &dtos.BackupRequest{
			Limit: "20",
		})
		if err != nil {
			logger.Error("failed to get backup checkpoints", zap.String("stackId", stackId.String()), zap.Error(err))
		} else {
			// Convert SDK recovery points to entity format
			recoveryPoints := make([]entities.RecoveryPoint, 0, len(backupCheckpoints))
			for _, checkpoint := range backupCheckpoints {
				// Convert SDK checkpoint to our recovery point structure
				checkpointBytes, err := json.Marshal(checkpoint)
				if err != nil {
					logger.Error("failed to marshal checkpoint", zap.String("stackId", stackId.String()), zap.Error(err))
					continue
				}

				var checkpointMap map[string]interface{}
				if json.Unmarshal(checkpointBytes, &checkpointMap) != nil {
					logger.Error("failed to unmarshal checkpoint", zap.String("stackId", stackId.String()), zap.Error(err))
					continue
				}

				recoveryPoint := entities.RecoveryPoint{
					Metadata: checkpointMap,
				}

				// Extract common fields if they exist
				if id, ok := checkpointMap["id"].(string); ok {
					recoveryPoint.ID = id
				}
				if name, ok := checkpointMap["name"].(string); ok {
					recoveryPoint.Name = name
				}
				if status, ok := checkpointMap["status"].(string); ok {
					recoveryPoint.Status = status
				}
				if description, ok := checkpointMap["description"].(string); ok {
					recoveryPoint.Description = description
				}
				if size, ok := checkpointMap["size"].(float64); ok {
					recoveryPoint.Size = int64(size)
				}
				if createdAtStr, ok := checkpointMap["created_at"].(string); ok {
					if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
						recoveryPoint.CreatedAt = createdAt
					}
				}

				recoveryPoints = append(recoveryPoints, recoveryPoint)
			}

			backupEntity.SnapshotList = recoveryPoints
		}

		// Update the database with the refreshed backup data
		if err := b.backupRepo.UpsertBackup(backupEntity); err != nil {
			logger.Error("failed to update backup data in database", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		logger.Info("backup data refresh completed successfully", zap.String("stackId", stackId.String()))
	})

	return nil
}

func (b *BackupManager) GetCheckpoints(ctx context.Context, stackId uuid.UUID, request dtos.BackupRequest) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "backup-checkpoints")
	thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
	if err != nil {
		logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	backupCheckpoints, err := thanos.GetListBackup(ctx, thanosSDK, &dtos.BackupRequest{
		Limit: request.Limit,
	})
	if err != nil {
		logger.Error("failed to get backup checkpoints", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    backupCheckpoints,
	}, nil
}

func (b *BackupManager) BackupSnapshot(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	b.taskManager.AddTask(fmt.Sprintf("backup-snapshot-%s", stackId), func(ctx context.Context) {
		logPath := utils.GetLogPath(stack.ID, "backup-snapshot")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		err = thanos.BackupSnapshot(ctx, thanosSDK)
		if err != nil {
			logger.Error("failed to backup snapshot", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *BackupManager) BackupRestore(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	b.taskManager.AddTask(fmt.Sprintf("backup-restore-%s", stackId), func(ctx context.Context) {
		logPath := utils.GetLogPath(stack.ID, "backup-restore")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		err = thanos.BackupRestore(ctx, thanosSDK)
		if err != nil {
			logger.Error("failed to backup restore", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *BackupManager) BackupConfigure(ctx context.Context, stackId uuid.UUID, request dtos.BackupConfigureRequest) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	b.taskManager.AddTask(fmt.Sprintf("backup-configure-%s", stackId), func(ctx context.Context) {
		logPath := utils.GetLogPath(stack.ID, "backup-configure")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		err = thanos.BackupConfigure(ctx, thanosSDK, &request)
		if err != nil {
			logger.Error("failed to backup configure", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *BackupManager) BackupAttach(ctx context.Context, stackId uuid.UUID, request dtos.BackupAttachRequest) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	b.taskManager.AddTask(fmt.Sprintf("backup-attach-%s", stackId), func(ctx context.Context) {
		logPath := utils.GetLogPath(stack.ID, "backup-attach")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		err = thanos.BackupAttach(ctx, thanosSDK, &request)
		if err != nil {
			logger.Error("failed to backup attach", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *BackupManager) BackupCleanup(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	b.taskManager.AddTask(fmt.Sprintf("backup-cleanup-%s", stackId), func(ctx context.Context) {
		logPath := utils.GetLogPath(stack.ID, "backup-cleanup")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		err = thanos.CleanupUnusedBackupResources(ctx, thanosSDK)
		if err != nil {
			logger.Error("failed to backup cleanup", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *BackupManager) getThanosClient(ctx context.Context, stack *entities.StackEntity, logPath string) (*thanosSDK.ThanosStack, error) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return nil, err
	}
	return thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
}
