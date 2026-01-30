package integrations

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/types"
	sdkUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"go.uber.org/zap"
)

const (
	installTimeout   = 30 * time.Minute
	uninstallTimeout = 20 * time.Minute
	cancelWaitTime   = 30 * time.Second
	maxLogFailures   = 10
	maxLogLength     = 10000
)

// DRBStoredConfig is the config stored in the integration entity (no secrets)
type DRBStoredConfig struct {
	NodeType        string `json:"nodeType"`        // "leader" or "regular"
	UseCurrentChain bool   `json:"useCurrentChain"`
	RPC             string `json:"rpc,omitempty"`
	ChainID         uint64 `json:"chainId,omitempty"`
	AWSRegion       string `json:"awsRegion"`
	DeploymentPath  string `json:"deploymentPath,omitempty"` // Used for system stacks without a deployment path
	DatabaseConfig  struct {
		Type     string `json:"type"`
		Username string `json:"username"`
	} `json:"databaseConfig"`
}

// DRBIntegration handles DRB installation and uninstallation
type DRBIntegration struct {
	stackRepo       StackRepo
	deploymentRepo  DeploymentRepo
	integrationRepo IntegrationRepo
	logRepo         LogRepo
	taskManager     TaskMgr
}

// NewDRBIntegration creates a new DRB integration handler
func NewDRBIntegration(
	stackRepo StackRepo,
	deploymentRepo DeploymentRepo,
	integrationRepo IntegrationRepo,
	logRepo LogRepo,
	taskManager TaskMgr,
) *DRBIntegration {
	return &DRBIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

// Install installs DRB for the given stack
func (d *DRBIntegration) Install(ctx context.Context, stackId uuid.UUID, req dtos.InstallDRBRequest) (*entities.Response, error) {
	// Log request without sensitive data
	logger.Info("DRB installation requested",
		zap.String("stackId", stackId.String()),
		zap.Bool("useCurrentChain", req.UseCurrentChain),
		zap.String("awsRegion", req.AWSConfig.Region),
	)

	stack, err := d.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return errInternal(err)
	}
	if stack == nil {
		return errNotFound("Stack not found")
	}
	if stack.Status != entities.StackStatusDeployed {
		return errBadRequest("Stack is not deployed yet. Please wait for it to finish")
	}

	// Check for existing active DRB integration
	integrations, err := d.integrationRepo.GetActiveIntegrations(stackId.String(), enum.IntegrationTypeDRB.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		return errInternal(err)
	}
	if len(integrations) > 0 {
		return errBadRequest("There is already an active DRB integration")
	}

	// If using current chain, get RPC and ChainID from stack metadata or use defaults for system stacks
	if req.UseCurrentChain {
		if stack.Metadata != nil && stack.Metadata.L2RpcUrl != "" {
			req.RPC = stack.Metadata.L2RpcUrl
			req.ChainID = uint64(stack.Metadata.L2ChainId)
		} else if stack.DeploymentPath == "" {
			// System stack (e.g., Thanos Sepolia) - use known defaults
			req.RPC = "https://rpc.thanos-sepolia.tokamak.network"
			req.ChainID = 111551119090
		} else {
			return errBadRequest("Stack does not have L2 RPC URL configured. Please use custom network configuration.")
		}
	}

	logPath := utils.GetLogPath(stack.ID, "drb")

	// For system stacks (no deployment path), create a dedicated path for DRB
	deploymentPath := stack.DeploymentPath
	if deploymentPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errInternal(fmt.Errorf("failed to get home directory: %w", err))
		}
		deploymentPath = fmt.Sprintf("%s/.trh/integrations/%s/drb", homeDir, stackId.String())
		if err := os.MkdirAll(deploymentPath, 0755); err != nil {
			return errInternal(fmt.Errorf("failed to create deployment directory: %w", err))
		}
		logger.Info("Created deployment path for system stack DRB", zap.String("path", deploymentPath))
	}

	// Store config (without secrets)
	storedConfig := DRBStoredConfig{
		NodeType:        req.NodeType,
		UseCurrentChain: req.UseCurrentChain,
		RPC:             req.RPC,
		ChainID:         req.ChainID,
		AWSRegion:       req.AWSConfig.Region,
		DeploymentPath:  deploymentPath,
	}
	storedConfig.DatabaseConfig.Type = req.DatabaseConfig.Type
	storedConfig.DatabaseConfig.Username = req.DatabaseConfig.Username
	configBytes, _ := json.Marshal(storedConfig)

	integration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeDRB.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  configBytes,
		LogPath: logPath,
	}

	if err := d.integrationRepo.CreateIntegration(integration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		return errInternal(err)
	}

	d.taskManager.AddTask(fmt.Sprintf("install-drb-%s", stackId.String()), func(ctx context.Context) {
		d.installTask(ctx, integration.ID, stack, req, logPath)
	})

	return okResponse("DRB installation started successfully")
}

// Uninstall uninstalls DRB for the given stack
func (d *DRBIntegration) Uninstall(ctx context.Context, stackId string) (*entities.Response, error) {
	logger.Info("DRB uninstallation requested", zap.String("stackId", stackId))

	stack, err := d.stackRepo.GetStackByID(stackId)
	if err != nil {
		return errInternal(err)
	}
	if stack == nil {
		return errNotFound("Stack not found")
	}

	integration, err := d.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeDRB.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		return errInternal(err)
	}
	if integration == nil {
		return errNotFound("DRB integration not found")
	}

	if err := d.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		return errInternal(err)
	}

	logPath := utils.GetLogPath(stack.ID, "uninstall-drb")
	d.taskManager.AddTask(fmt.Sprintf("uninstall-drb-%s", stackId), func(ctx context.Context) {
		d.uninstallTask(ctx, integration.ID, stack, logPath)
	})

	return okResponse("DRB uninstallation started successfully")
}

// GetInfo returns DRB deployment information and status
func (d *DRBIntegration) GetInfo(ctx context.Context, stackId string) (*entities.Response, error) {
	logger.Info("DRB info requested", zap.String("stackId", stackId))

	stack, err := d.stackRepo.GetStackByID(stackId)
	if err != nil {
		return errInternal(err)
	}
	if stack == nil {
		return errNotFound("Stack not found")
	}

	// Get the DRB integration for this stack
	integrations, err := d.integrationRepo.GetActiveIntegrations(stack.ID.String(), enum.IntegrationTypeDRB.String())
	if err != nil {
		logger.Error("failed to get integrations", zap.Error(err))
		return errInternal(err)
	}

	// No DRB integration found
	if len(integrations) == 0 {
		return &entities.Response{
			Status:  200,
			Message: "DRB not installed",
			Data: &dtos.GetDRBInfoResponse{
				Status:  "not_installed",
				Message: "DRB has not been installed on this stack",
			},
		}, nil
	}

	integration := integrations[0]
	response := &dtos.GetDRBInfoResponse{}

	// Map integration status to response status
	switch integration.Status {
	case string(entities.DeploymentStatusPending):
		response.Status = "pending"
		response.Message = "DRB installation is pending"
	case string(entities.DeploymentStatusInProgress):
		response.Status = "in_progress"
		response.Message = "DRB installation is in progress"
	case string(entities.DeploymentStatusCompleted):
		response.Status = "installed"
		response.Message = "DRB is installed and running"
	case string(entities.DeploymentStatusFailed):
		response.Status = "failed"
		response.Message = "DRB installation failed"
		response.FailureReason = integration.Reason
	case string(entities.DeploymentStatusTerminating):
		response.Status = "terminating"
		response.Message = "DRB is being uninstalled"
	case string(entities.DeploymentStatusCancelling):
		response.Status = "cancelling"
		response.Message = "DRB installation is being cancelled"
	case string(entities.DeploymentStatusCancelled):
		response.Status = "cancelled"
		response.Message = "DRB installation was cancelled"
	default:
		response.Status = integration.Status
	}

	// Get stored config for node type
	if integration.Config != nil {
		var storedConfig DRBStoredConfig
		if err := json.Unmarshal(integration.Config, &storedConfig); err == nil {
			response.NodeType = storedConfig.NodeType
			if response.NodeType == "" {
				response.NodeType = "leader" // default for older installations
			}
		}
	}

	// Get deployment metadata if installed
	if integration.Status == string(entities.DeploymentStatusCompleted) && integration.Info != nil {
		var deploymentInfo dtos.DRBDeploymentInfo
		if err := json.Unmarshal(integration.Info, &deploymentInfo); err == nil {
			response.Deployment = &deploymentInfo
			// Ensure node type is set on deployment info too
			if response.Deployment.NodeType == "" {
				response.Deployment.NodeType = response.NodeType
			}
		}
	}

	return &entities.Response{
		Status:  200,
		Message: "DRB info retrieved successfully",
		Data:    response,
	}, nil
}

// Cancel cancels an in-progress DRB installation
func (d *DRBIntegration) Cancel(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	logger.Info("DRB cancellation requested",
		zap.String("stackId", stackId.String()),
		zap.String("integrationId", integrationId.String()),
	)

	integration, err := d.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		logger.Error("failed to get integration", zap.Error(err), zap.String("integrationId", integrationId.String()))
		return errInternal(err)
	}
	if integration == nil {
		return errNotFound("Integration not found")
	}

	// Can only cancel if in progress or pending
	if integration.Status != string(entities.DeploymentStatusInProgress) && integration.Status != string(entities.DeploymentStatusPending) {
		return errBadRequest("Can only cancel installations that are in progress or pending")
	}

	if err = d.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelling,
		"Stopping installation process. This may take a few minutes to safely clean up resources.",
	); err != nil {
		return errInternal(err)
	}

	d.taskManager.AddTask(fmt.Sprintf("cancel-drb-%s", stackId.String()), func(ctx context.Context) {
		d.cancelTask(ctx, stackId, integration)
	})

	return okResponse("Cancellation in progress. Installation will be stopped and resources will be cleaned up.")
}

// Retry is not fully supported for DRB (requires re-entering credentials)
func (d *DRBIntegration) Retry(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return errBadRequest("DRB retry requires re-entering credentials. Please uninstall and install again.")
}

// installTask handles the actual installation process
func (d *DRBIntegration) installTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, req dtos.InstallDRBRequest, logPath string) {
	taskCtx, taskCancel := context.WithTimeout(ctx, installTimeout)
	defer taskCancel()

	stackConfig, err := d.parseStackConfig(stack)
	if err != nil {
		d.failIntegrationOnly(integrationID, "Failed to parse stack configuration")
		return
	}

	// Get stored config for deployment path (needed for system stacks)
	deploymentPath := d.resolveDeploymentPath(stack, &integrationID)

	if err := d.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		return
	}

	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallDRBStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := d.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		d.failIntegrationOnly(integrationID, "Failed to create deployment record")
		return
	}

	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go d.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

	// Resolve AWS credentials (prefer request config for system stacks)
	awsCreds := d.resolveAWSCredentials(stackConfig, req.AWSConfig, nil)

	sdkClient, err := thanos.NewThanosSDKClient(
		taskCtx, logPath, string(stack.Network), deploymentPath,
		stackConfig.RegisterCandidate, awsCreds.AccessKey, awsCreds.SecretKey, awsCreds.Region,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		d.failIntegration(integrationID, deployment.ID, "Failed to initialize SDK client")
		return
	}

	// Install DRB based on node type
	var drbOutput *thanosTypes.DeployDRBOutput
	if req.NodeType == "regular" {
		// Regular node installation
		err = thanos.InstallDRBRegular(taskCtx, sdkClient, &req)
	} else {
		// Leader node installation (default)
		drbOutput, err = thanos.InstallDRBLeader(taskCtx, sdkClient, &req)
	}
	if err != nil {
		logger.Error("failed to install DRB", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.String("nodeType", req.NodeType), zap.Error(err))
		status := entities.DeploymentRunStatusFailed
		reason := sanitizeErrorMessage(err.Error())
		if errors.Is(err, context.Canceled) {
			status = entities.DeploymentRunStatusStopped
			reason = "Installation was cancelled"
		} else if errors.Is(err, context.DeadlineExceeded) {
			reason = "Installation timed out after 30 minutes"
		}
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, reason)
		_ = d.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), status)
		return
	}

	logger.Info("DRB successfully installed",
		zap.String("plugin", enum.IntegrationTypeDRB.String()),
		zap.String("integrationId", integrationID.String()),
	)

	// Build metadata
	drbMetadata := &dtos.DRBDeploymentInfo{
		NodeType:     req.NodeType,
		DatabaseType: req.DatabaseConfig.Type,
	}
	if drbOutput != nil {
		if drbOutput.DeployDRBContractsOutput != nil {
			drbMetadata.Contract = &dtos.DRBContractInfo{
				ContractAddress:          drbOutput.DeployDRBContractsOutput.ContractAddress,
				ContractName:             drbOutput.DeployDRBContractsOutput.ContractName,
				ChainID:                  drbOutput.DeployDRBContractsOutput.ChainID,
				ConsumerExampleV2Address: drbOutput.DeployDRBContractsOutput.ConsumerExampleV2Address,
			}
		}
		if drbOutput.DeployDRBApplicationOutput != nil {
			drbMetadata.Application = &dtos.DRBApplicationInfo{
				LeaderNodeURL: drbOutput.DeployDRBApplicationOutput.LeaderNodeURL,
			}
		}
	}

	// Read additional deployment info based on node type
	if req.NodeType == "regular" {
		nodeEOA := ""
		if addr, err := sdkUtils.GetAddressFromPrivateKey(req.EOAPrivateKey); err == nil {
			nodeEOA = addr.Hex()
		}

		drbMetadata.RegularNodeInfo = &dtos.DRBRegularNodeInfo{
			NodePort:            req.NodePort,
			NodeEOA:             nodeEOA,
			Region:              req.AWSConfig.Region,
			ChainID:             req.ChainID,
			RPCURL:              req.RPC,
			LeaderIP:            req.LeaderIP,
			LeaderPort:          req.LeaderPort,
			LeaderPeerID:        req.LeaderPeerID,
			LeaderEOA:           req.LeaderEOA,
			ContractAddress:     req.ContractAddress,
			DeploymentTimestamp: time.Now().UTC().Format(time.RFC3339),
		}
		if req.EC2Config != nil {
			drbMetadata.RegularNodeInfo.InstanceType = req.EC2Config.InstanceType
		}
		logger.Info("DRB regular node info saved",
			zap.String("region", req.AWSConfig.Region),
			zap.Int("nodePort", req.NodePort),
			zap.String("nodeEOA", nodeEOA),
		)
	} else {
		// Read the leader info file for additional connection details
		leaderInfoPath := fmt.Sprintf("%s/drb-leader-info.json", deploymentPath)
		if leaderInfoData, err := os.ReadFile(leaderInfoPath); err == nil {
			var leaderInfo dtos.DRBLeaderInfo
			if err := json.Unmarshal(leaderInfoData, &leaderInfo); err == nil {
				drbMetadata.LeaderInfo = &leaderInfo
				logger.Info("DRB leader info loaded",
					zap.String("leaderUrl", leaderInfo.LeaderURL),
					zap.String("leaderPeerId", leaderInfo.LeaderPeerID),
				)
			} else {
				logger.Warn("failed to parse DRB leader info", zap.Error(err))
			}
		} else {
			logger.Warn("failed to read DRB leader info file", zap.Error(err), zap.String("path", leaderInfoPath))
		}
	}

	metadataBytes, err := json.Marshal(drbMetadata)
	if err != nil {
		logger.Error("failed to marshal DRB metadata", zap.Error(err))
		d.failIntegration(integrationID, deployment.ID, "Failed to save deployment metadata")
		return
	}

	if err = d.integrationRepo.UpdateMetadataAfterInstalled(integrationID.String(), entities.IntegrationInfo(metadataBytes)); err != nil {
		logger.Error("failed to update integration metadata", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		d.failIntegration(integrationID, deployment.ID, "Failed to update integration metadata")
		return
	}

	_ = d.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// uninstallTask handles the actual uninstallation process
func (d *DRBIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, logPath string) {
	// Add timeout for uninstall
	taskCtx, taskCancel := context.WithTimeout(ctx, uninstallTimeout)
	defer taskCancel()

	stackConfig, err := d.parseStackConfig(stack)
	if err != nil {
		d.failIntegrationOnly(integrationID, "Failed to parse stack configuration")
		return
	}

	var deployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during DRB uninstall",
				zap.String("plugin", enum.IntegrationTypeDRB.String()),
				zap.Any("recover", r),
				zap.String("stackId", stack.ID.String()),
				zap.String("integrationId", integrationID.String()),
				zap.String("deploymentPath", stack.DeploymentPath),
			)
			if deployment != nil {
				_ = d.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			reason := fmt.Sprintf("Panic during uninstall: %v. AWS resources may require manual cleanup.", r)
			_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, reason)
		}
	}()

	if err := d.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		return
	}

	deployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallDRBStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := d.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.Error(err))
		d.failIntegrationOnly(integrationID, "Failed to create deployment record")
		return
	}

	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go d.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

	awsCreds := d.resolveAWSCredentials(stackConfig, nil, &integrationID)
	deploymentPath := d.resolveDeploymentPath(stack, &integrationID)

	// Get stored config to determine node type
	nodeType := "leader" // default to leader
	if integration, err := d.integrationRepo.GetIntegrationById(integrationID.String()); err == nil && integration != nil && integration.Config != nil {
		var storedConfig DRBStoredConfig
		if err := json.Unmarshal(integration.Config, &storedConfig); err == nil && storedConfig.NodeType != "" {
			nodeType = storedConfig.NodeType
		}
	}

	sdkClient, err := thanos.NewThanosSDKClient(
		taskCtx, logPath, string(stack.Network), deploymentPath,
		stackConfig.RegisterCandidate, awsCreds.AccessKey, awsCreds.SecretKey, awsCreds.Region,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		d.failIntegration(integrationID, deployment.ID, "Failed to initialize SDK client")
		return
	}

	// Uninstall based on node type
	if nodeType == "regular" {
		err = thanos.UninstallDRBRegular(taskCtx, sdkClient)
	} else {
		err = thanos.UninstallDRBLeader(taskCtx, sdkClient)
	}
	if err != nil {
		logger.Error("failed to uninstall DRB", zap.String("plugin", enum.IntegrationTypeDRB.String()), zap.String("nodeType", nodeType), zap.Error(err))
		reason := sanitizeErrorMessage(err.Error())
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "Uninstall timed out after 20 minutes. AWS resources may require manual cleanup."
		}
		_ = d.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, reason)
		return
	}

	logger.Info("DRB successfully uninstalled", zap.String("integrationId", integrationID.String()))
	_ = d.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated)
	_ = d.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// cancelTask handles the cancellation cleanup
func (d *DRBIntegration) cancelTask(ctx context.Context, stackId uuid.UUID, integration *entities.IntegrationEntity) {
	taskId := fmt.Sprintf("install-drb-%s", stackId.String())

	// Stop the running install task
	d.taskManager.StopTask(taskId)

	// Wait for task to actually stop (with timeout)
	deadline := time.Now().Add(cancelWaitTime)
	for time.Now().Before(deadline) {
		if !d.taskManager.IsTaskRunning(taskId) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Small buffer to ensure cleanup completes
	time.Sleep(2 * time.Second)

	stack, err := d.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.Error(err), zap.String("stackId", stackId.String()))
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, "Failed to get stack for cleanup")
		return
	}

	stackConfig, err := d.parseStackConfig(stack)
	if err != nil {
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, "Failed to parse stack config for cleanup")
		return
	}

	awsCreds := d.resolveAWSCredentials(stackConfig, nil, &integration.ID)
	deploymentPath := d.resolveDeploymentPath(stack, &integration.ID)

	// Get node type from stored config
	nodeType := "leader" // default to leader
	if integration.Config != nil {
		var storedConfig DRBStoredConfig
		if err := json.Unmarshal(integration.Config, &storedConfig); err == nil && storedConfig.NodeType != "" {
			nodeType = storedConfig.NodeType
		}
	}

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx, utils.GetLogPath(stack.ID, "cancel-drb"), string(stack.Network), deploymentPath,
		stackConfig.RegisterCandidate, awsCreds.AccessKey, awsCreds.SecretKey, awsCreds.Region,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, "Failed to initialize SDK for cleanup")
		return
	}

	// Uninstall based on node type
	if nodeType == "regular" {
		err = thanos.UninstallDRBRegular(ctx, sdkClient)
	} else {
		err = thanos.UninstallDRBLeader(ctx, sdkClient)
	}
	if err != nil {
		logger.Error("failed to uninstall DRB during cancel", zap.String("nodeType", nodeType), zap.Error(err))
		_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, sanitizeErrorMessage(err.Error()))
		return
	}

	logger.Info("Cancellation completed successfully", zap.String("integrationId", integration.ID.String()))
	_ = d.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelled,
		"Installation cancelled successfully. All resources have been cleaned up.",
	)
}

// Helper methods

type awsCredentials struct {
	AccessKey string
	SecretKey string
	Region    string
}

func (d *DRBIntegration) resolveDeploymentPath(stack *entities.StackEntity, integrationID *uuid.UUID) string {
	// First try the stack's deployment path
	if stack.DeploymentPath != "" {
		return stack.DeploymentPath
	}

	// For system stacks, get the deployment path from stored integration config
	if integrationID != nil {
		if integration, err := d.integrationRepo.GetIntegrationById(integrationID.String()); err == nil && integration != nil && integration.Config != nil {
			var config DRBStoredConfig
			if err := json.Unmarshal(integration.Config, &config); err == nil && config.DeploymentPath != "" {
				return config.DeploymentPath
			}
		}
	}

	// Fallback: create a new path (shouldn't happen if Install was called correctly)
	homeDir, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.trh/integrations/%s/drb", homeDir, stack.ID.String())
}

func (d *DRBIntegration) resolveAWSCredentials(stackConfig *dtos.DeployThanosRequest, reqConfig *dtos.DRBAWSConfig, integrationID *uuid.UUID) awsCredentials {
	creds := awsCredentials{
		AccessKey: stackConfig.AwsAccessKey,
		SecretKey: stackConfig.AwsSecretAccessKey,
		Region:    stackConfig.AwsRegion,
	}

	// Use request config if stack doesn't have credentials (e.g., system stack)
	if reqConfig != nil && creds.AccessKey == "" {
		creds.AccessKey = reqConfig.AccessKeyId
		creds.SecretKey = reqConfig.SecretAccessKey
		creds.Region = reqConfig.Region
	}

	// Fallback to stored integration config for region
	if integrationID != nil && creds.Region == "" {
		if integration, err := d.integrationRepo.GetIntegrationById(integrationID.String()); err == nil && integration != nil {
			var config DRBStoredConfig
			if err := json.Unmarshal(integration.Config, &config); err == nil && config.AWSRegion != "" {
				creds.Region = config.AWSRegion
			}
		}
	}

	return creds
}

func (d *DRBIntegration) parseStackConfig(stack *entities.StackEntity) (*dtos.DeployThanosRequest, error) {
	var config dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &config); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return nil, err
	}
	return &config, nil
}

func (d *DRBIntegration) failIntegration(integrationID, deploymentID uuid.UUID, reason string) {
	_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, reason)
	_ = d.deploymentRepo.UpdateDeploymentStatus(deploymentID.String(), entities.DeploymentRunStatusFailed)
}

func (d *DRBIntegration) failIntegrationOnly(integrationID uuid.UUID, reason string) {
	_ = d.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, reason)
}

func (d *DRBIntegration) tailAndIngestLogs(ctx context.Context, stackID, deploymentID uuid.UUID, logPath string) {
	// Wait for file to appear
	for {
		if _, err := os.Stat(logPath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		logger.Error("failed to open log file", zap.String("path", logPath), zap.Error(err))
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	failureCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if msg := strings.TrimRight(line, "\r\n"); msg != "" {
				sanitizedMsg := sanitizeLogMessage(msg)
				if dbErr := d.logRepo.CreateLog(&entities.LogEntity{
					StackID:      &stackID,
					DeploymentID: &deploymentID,
					Message:      sanitizedMsg,
				}); dbErr != nil {
					logger.Error("failed to insert log", zap.Error(dbErr))
					failureCount++
					if failureCount >= maxLogFailures {
						logger.Error("too many log insertion failures, stopping ingestion",
							zap.String("logPath", logPath),
							zap.Int("failureCount", failureCount),
						)
						return
					}
				} else {
					failureCount = 0 // Reset on success
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(300 * time.Millisecond)
					continue
				}
				logger.Error("error reading log file", zap.Error(err))
				return
			}
		}
	}
}

// sanitizeLogMessage removes control characters and limits length
func sanitizeLogMessage(msg string) string {
	// Remove control characters except tabs and newlines
	sanitized := strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, msg)

	// Limit length
	if len(sanitized) > maxLogLength {
		sanitized = sanitized[:maxLogLength] + "... (truncated)"
	}

	return sanitized
}

// sanitizeErrorMessage removes potentially sensitive info from error messages
func sanitizeErrorMessage(msg string) string {
	sanitized := msg

	// Remove AWS access key IDs (AKIA...)
	awsKeyPattern := regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	sanitized = awsKeyPattern.ReplaceAllString(sanitized, "[AWS_ACCESS_KEY_REDACTED]")

	// Remove AWS secret keys (40 character base64-like strings often near "secret")
	secretPattern := regexp.MustCompile(`(?i)(secret[_\s]*(?:access)?[_\s]*key[_\s]*[:=]?\s*)([A-Za-z0-9/+=]{40})`)
	sanitized = secretPattern.ReplaceAllString(sanitized, "$1[REDACTED]")

	// Remove private keys (64 hex chars)
	privateKeyPattern := regexp.MustCompile(`(0x)?[a-fA-F0-9]{64}`)
	sanitized = privateKeyPattern.ReplaceAllString(sanitized, "[PRIVATE_KEY_REDACTED]")

	// Remove database passwords in connection strings
	dbPasswordPattern := regexp.MustCompile(`(postgres://[^:]+:)([^@]+)(@)`)
	sanitized = dbPasswordPattern.ReplaceAllString(sanitized, "$1[REDACTED]$3")

	// Truncate very long error messages
	if len(sanitized) > 500 {
		sanitized = sanitized[:500] + "..."
	}

	return sanitized
}
