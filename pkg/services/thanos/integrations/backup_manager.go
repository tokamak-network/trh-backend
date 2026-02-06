package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosSDK "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/types"
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

	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64)))
		SetTaskResult(id string, result any)
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
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64)))
		SetTaskResult(id string, result any)
	},
) *BackupManager {
	return &BackupManager{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
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
	opCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	backupStatus, err := thanos.GetBackupStatus(opCtx, thanosSDK)
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
	opCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	backupCheckpoints, err := thanos.GetListBackup(opCtx, thanosSDK, &dtos.BackupRequest{
		Limit: "20",
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

	taskId := fmt.Sprintf("backup-snapshot-%s", stackId)
	b.taskManager.AddTaskWithProgress(taskId, func(ctx context.Context, updateProgress func(string, float64)) {
		logPath := utils.GetLogPath(stack.ID, "backup-snapshot")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			updateProgress("Failed to get Thanos client", 0)
			return
		}
		snapshotInfo, err := BackupSnapshot(ctx, thanosSDK, updateProgress)
		if err != nil {
			logger.Error("failed to backup snapshot", zap.String("stackId", stackId.String()), zap.Error(err))
			updateProgress(fmt.Sprintf("Snapshot failed: %v", err), 0)
			return
		}
		logger.Info("backup snapshot info", zap.Any("backup snapshot info", snapshotInfo))
		updateProgress("Snapshot completed successfully", 100)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully initiated snapshot",
		Data:    gin.H{"task_id": taskId},
	}, nil
}

func (b *BackupManager) BackupRestore(ctx context.Context, stackId uuid.UUID, request dtos.BackupRestoreRequest) (*entities.Response, error) {
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

	taskId := fmt.Sprintf("backup-restore-%s", stackId)
	b.taskManager.AddTaskWithProgress(taskId, func(ctx context.Context, updateProgress func(string, float64)) {
		logPath := utils.GetLogPath(stack.ID, "backup-restore")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			updateProgress("Failed to get Thanos client", 0)
			return
		}
		backupRestoreInfo, err := BackupRestore(ctx, thanosSDK, request, updateProgress)
		if err != nil {
			logger.Error("failed to backup restore", zap.String("stackId", stackId.String()), zap.Error(err))
			updateProgress(fmt.Sprintf("Restore failed: %v", err), 0)
			return
		}
		logger.Info("backup restore info", zap.Any("backup restore info", backupRestoreInfo))
		b.taskManager.SetTaskResult(taskId, backupRestoreInfo)
		// Ensure progress is 100% on success if not already
		updateProgress("Restore completed successfully", 100)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully initiated restore",
		Data:    gin.H{"task_id": taskId},
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
		backupConfigureInfo, err := BackupConfigure(ctx, thanosSDK, &request)
		if err != nil {
			logger.Error("failed to backup configure", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		logger.Info("backup configure info", zap.Any("backup configure info", backupConfigureInfo))
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

	taskId := fmt.Sprintf("backup-attach-%s", stackId)
	b.taskManager.AddTaskWithProgress(taskId, func(ctx context.Context, updateProgress func(string, float64)) {
		updateProgress("Starting attach...", 5)
		logPath := utils.GetLogPath(stack.ID, "backup-attach")
		thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
		if err != nil {
			logger.Error("failed to get thanos client", zap.String("stackId", stackId.String()), zap.Error(err))
			updateProgress("Failed to get Thanos client", 0)
			return
		}
		updateProgress("Thanos client ready", 15)
		if request.BackupPvPvc == nil {
			defaultBackup := true
			request.BackupPvPvc = &defaultBackup
		}
		backupAttachInfo, err := BackupAttach(ctx, thanosSDK, &request, updateProgress)
		if err != nil {
			logger.Error("failed to backup attach", zap.String("stackId", stackId.String()), zap.Error(err))
			updateProgress(fmt.Sprintf("Attach failed: %v", err), 0)
			return
		}
		logger.Info("backup attach info", zap.Any("backup attach info", backupAttachInfo))
		b.taskManager.SetTaskResult(taskId, backupAttachInfo)
		updateProgress("Attach completed successfully", 100)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    gin.H{"task_id": taskId},
	}, nil
}

// BackupPvPvcExport generates PV/PVC backup artifacts and returns a zip file path and filename.
func (b *BackupManager) BackupPvPvcExport(ctx context.Context, stackId uuid.UUID) (string, string, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return "", "", err
	}
	if stack == nil {
		return "", "", fmt.Errorf("stack not found")
	}
	if stack.Status != entities.StackStatusDeployed {
		return "", "", fmt.Errorf("stack is not deployed")
	}

	logPath := utils.GetLogPath(stack.ID, "backup-pv-pvc-export")
	thanosSDK, err := b.getThanosClient(ctx, stack, logPath)
	if err != nil {
		return "", "", err
	}

	backupDir, err := thanosSDK.BackupPvPvcExport(ctx)
	if err != nil {
		return "", "", err
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("trh-pvpvc-%s-*.zip", stackId.String()[:8]))
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp zip file: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close temp zip file: %w", err)
	}

	if err := utils.ZipDirectory(backupDir, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("failed to zip backup directory: %w", err)
	}

	filename := fmt.Sprintf("pvpvc-backup-%s.zip", stackId.String()[:8])
	return tmpPath, filename, nil
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
		err = CleanupUnusedBackupResources(ctx, thanosSDK)
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

	opCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	return thanos.NewThanosSDKClient(
		opCtx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
}

func GetBackupStatus(ctx context.Context, s *thanosStack.ThanosStack) (*thanosTypes.BackupStatusInfo, error) {
	backupRestoreInfo, err := s.BackupStatus(ctx)
	if err != nil {
		return nil, err
	}
	return backupRestoreInfo, nil
}

func BackupRestore(ctx context.Context, s *thanosStack.ThanosStack, request dtos.BackupRestoreRequest, progressReporter func(string, float64)) (*thanosTypes.BackupRestoreInfo, error) {
	backupRestoreInfo, err := s.BackupRestore(ctx, request.RecoveryPointID, request.Attach, request.Pvcs, request.Stss, progressReporter)
	if err != nil {
		return nil, err
	}
	return backupRestoreInfo, nil
}

func BackupSnapshot(ctx context.Context, s *thanosStack.ThanosStack, progressReporter func(string, float64)) (*thanosTypes.BackupSnapshotInfo, error) {
	snapshotInfo, err := s.BackupSnapshot(ctx, progressReporter)
	if err != nil {
		return nil, err
	}
	return snapshotInfo, nil
}

func BackupAttach(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.BackupAttachRequest, progressReporter func(string, float64)) (*thanosTypes.BackupAttachInfo, error) {
	backupAttachInfo, err := s.BackupAttach(ctx, req.EfsId, req.Pvcs, req.Stss, req.BackupPvPvc, progressReporter)
	if err != nil {
		return nil, err
	}
	return backupAttachInfo, nil
}

func BackupConfigure(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.BackupConfigureRequest) (*thanosTypes.BackupConfigInfo, error) {
	backupConfigureInfo, err := s.BackupConfigure(ctx, req.Daily, req.Keep, req.Reset)
	if err != nil {
		return nil, err
	}
	return backupConfigureInfo, nil
}

func CleanupUnusedBackupResources(ctx context.Context, s *thanosStack.ThanosStack) error {
	err := s.CleanupUnusedBackupResources(ctx)
	if err != nil {
		return err
	}
	return nil
}
