package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
	"go.uber.org/zap"
)

// BuildDefaultCrossTradeL2L1Request builds an InstallCrossChainBridgeRequest for L2_TO_L1 mode
// with a single L2 chain.
func BuildDefaultCrossTradeL2L1Request(
	l1RPC string,
	l1ChainID uint64,
	l2RPC string,
	l2ChainID uint64,
	privateKey string,
	projectID string,
) *dtos.InstallCrossChainBridgeRequest {
	// Ensure private key has 0x prefix
	if len(privateKey) > 0 && !strings.HasPrefix(privateKey, "0x") {
		privateKey = "0x" + privateKey
	}

	l1ChainConfig := &dtos.L1CrossTradeChainInput{
		RPC:           l1RPC,
		ChainID:       l1ChainID,
		PrivateKey:    privateKey,
		ChainName:     "Ethereum Sepolia",
		IsDeployedNew: true,
	}

	l2ChainConfig := &dtos.L2CrossTradeChainInput{
		RPC:                     l2RPC,
		ChainID:                 l2ChainID,
		PrivateKey:              privateKey,
		IsDeployedNew:           true,
		ChainName:               "L2 Chain",
		CrossDomainMessenger:    "0x4200000000000000000000000000000000000007",
		NativeTokenAddress:      "0x0000000000000000000000000000000000000000",
		L1StandardBridgeAddress: "0x0000000000000000000000000000000000000000",
		L1USDCBridgeAddress:     "0x0000000000000000000000000000000000000000",
		L1CrossDomainMessenger:  "0x0000000000000000000000000000000000000000",
	}

	return &dtos.InstallCrossChainBridgeRequest{
		Mode:          thanosConstants.CrossTradeDeployModeL2ToL1,
		ProjectID:     projectID,
		L1ChainConfig: l1ChainConfig,
		L2ChainConfig: []*dtos.L2CrossTradeChainInput{l2ChainConfig},
	}
}

// BuildDefaultCrossTradeL2L2Request builds an InstallCrossChainBridgeRequest for L2_TO_L2 mode
// with the custom L2 (IsDeployedNew: true) plus three default Sepolia L2 chains (Optimism, Base, Unichain).
// The custom L2 requires L1 bridge addresses to configure setChainInfo on L1.
func BuildDefaultCrossTradeL2L2Request(
	l1RPC string,
	l1ChainID uint64,
	l2RPC string,
	l2ChainID uint64,
	l1StandardBridge string,
	l1USDCBridge string,
	l1CrossDomainMessenger string,
	privateKey string,
	projectID string,
) *dtos.InstallCrossChainBridgeRequest {
	// Ensure private key has 0x prefix
	if len(privateKey) > 0 && !strings.HasPrefix(privateKey, "0x") {
		privateKey = "0x" + privateKey
	}

	l1ChainConfig := &dtos.L1CrossTradeChainInput{
		RPC:           l1RPC,
		ChainID:       l1ChainID,
		PrivateKey:    privateKey,
		ChainName:     "Ethereum Sepolia",
		IsDeployedNew: true,
	}

	// Custom L2: deploy new L2toL2CrossTradeProxy via L1 deposit tx
	customL2 := &dtos.L2CrossTradeChainInput{
		RPC:                     l2RPC,
		ChainID:                 l2ChainID,
		PrivateKey:              privateKey,
		IsDeployedNew:           true,
		ChainName:               "Custom L2 Chain",
		CrossDomainMessenger:    "0x4200000000000000000000000000000000000007",
		NativeTokenAddress:      "0x0000000000000000000000000000000000000000",
		L1StandardBridgeAddress: l1StandardBridge,
		L1USDCBridgeAddress:     l1USDCBridge,
		L1CrossDomainMessenger:  l1CrossDomainMessenger,
	}

	l2ChainConfigs := []*dtos.L2CrossTradeChainInput{customL2}

	// Three external Sepolia L2 chains (IsDeployedNew: false — use existing deployed contracts)
	sepoliaChainIDs := []uint64{
		thanosConstants.OptimismSepoliaChainID, // 11155420
		thanosConstants.BaseSepoliaChainID,     // 84532
		thanosConstants.UnichainSepoliaChainID, // 1301
	}
	for _, chainID := range sepoliaChainIDs {
		addresses, exists := thanosConstants.DefaultContractAddresses[chainID]
		if !exists {
			logger.Warn("DefaultContractAddresses not found for chain", zap.Uint64("chainID", chainID))
			continue
		}

		l2ChainConfig := &dtos.L2CrossTradeChainInput{
			RPC:                     "",
			ChainID:                 chainID,
			PrivateKey:              privateKey,
			IsDeployedNew:           false,
			ChainName:               fmt.Sprintf("Sepolia L2 Chain %d", chainID),
			CrossDomainMessenger:    addresses.L2CrossDomainMessengerAddress,
			NativeTokenAddress:      addresses.NativeTokenAddress,
			L1StandardBridgeAddress: addresses.L1StandardBridgeAddress,
			L1USDCBridgeAddress:     addresses.L1USDCBridgeAddress,
			L1CrossDomainMessenger:  addresses.L1CrossDomainMessengerAddress,
		}
		l2ChainConfigs = append(l2ChainConfigs, l2ChainConfig)
	}

	return &dtos.InstallCrossChainBridgeRequest{
		Mode:          thanosConstants.CrossTradeDeployModeL2ToL2,
		ProjectID:     projectID,
		L1ChainConfig: l1ChainConfig,
		L2ChainConfig: l2ChainConfigs,
	}
}

// CrossTradeAWSConfig stores the composite configuration for the two-phase AWS auto-install.
type CrossTradeAWSConfig struct {
	L2ToL1 *dtos.InstallCrossChainBridgeRequest `json:"l2ToL1,omitempty"`
	L2ToL2 *dtos.InstallCrossChainBridgeRequest `json:"l2ToL2,omitempty"`
}

// autoInstallCrossTradeAWS is a goroutine that runs L2_TO_L1 then L2_TO_L2 sequentially
// for AWS K8s deployments with DeFi/Full presets.
// It manages a single "cross-trade" integration DB record with the pending status.
// This is called from deployment.go after AWS infrastructure is deployed.
func (b *CrossTradeBridgeIntegration) autoInstallCrossTradeAWS(
	ctx context.Context,
	stackId uuid.UUID,
	stackConfig *dtos.DeployThanosRequest,
	l2RPC string,
	l2ChainID uint64,
	l1ChainID uint64,
) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack by id",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	pendingIntegration, err := b.integrationRepo.GetIntegrationByStatus(
		stackId.String(),
		enum.IntegrationTypeCrossTrade.String(),
		entities.DeploymentStatusPending,
	)
	if err != nil {
		logger.Error("failed to get pending cross-trade integration",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	if pendingIntegration == nil {
		logger.Warn("no pending cross-trade integration found",
			zap.String("stackId", stackId.String()))
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during auto-install cross trade AWS",
				zap.String("stackId", stackId.String()),
				zap.Any("recover", r))
			if err := b.integrationRepo.UpdateIntegrationStatusWithReason(
				pendingIntegration.ID.String(),
				entities.DeploymentStatusFailed,
				fmt.Sprintf("panic: %v", r),
			); err != nil {
				logger.Error("failed to update integration status after panic", zap.Error(err))
			}
		}
	}()

	if err := b.integrationRepo.UpdateIntegrationStatus(
		pendingIntegration.ID.String(),
		entities.DeploymentStatusInProgress,
	); err != nil {
		logger.Error("failed to update integration status to in-progress",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	// Deploy L2 contracts via L1 deposit-tx + install both Helm releases (L2→L1 and L2→L2).
	// This path does not require pre-funded L2 ETH — contrast with InstallCrossTradeBridge
	// which uses forge --broadcast and requires L2 ETH that a fresh chain never has.
	logPath := utils.GetLogPath(stackId, "install-cross-trade-bridge")
	if mkdirErr := os.MkdirAll(filepath.Dir(logPath), 0755); mkdirErr != nil {
		logger.Warn("failed to create log directory for CrossTrade auto-install",
			zap.String("stackId", stackId.String()),
			zap.Error(mkdirErr))
	}

	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallCrossTradeBridgeStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	deploymentCreated := false
	if createErr := b.deploymentRepo.CreateDeployment(deployment); createErr != nil {
		logger.Warn("failed to create deployment record for CrossTrade auto-install",
			zap.String("stackId", stackId.String()),
			zap.Error(createErr))
	} else {
		deploymentCreated = true
		if f, fErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); fErr == nil {
			fmt.Fprintf(f, "Chain deployment completed. Starting CrossTrade auto-installation...\n")
			f.Close()
		}
		ingestCtx, cancelIngest := context.WithCancel(ctx)
		defer cancelIngest()
		go b.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)
	}

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		false,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create SDK client for CrossTrade AWS",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if deploymentCreated {
			_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		}
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	output, err := thanos.AutoInstallCrossTradeAWS(ctx, sdkClient)
	if err != nil {
		logger.Error("failed to auto-install CrossTrade AWS",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if deploymentCreated {
			_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		}
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	if deploymentCreated {
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
	}

	logger.Info("CrossTrade AWS installed successfully",
		zap.String("stackId", stackId.String()),
		zap.String("l2l1URL", output.L2L1DAppURL),
		zap.String("l2l2URL", output.L2L2DAppURL))

	contracts := map[string]string{
		"l2_cross_trade_proxy":       output.L2CrossTradeProxy,
		"l1_cross_trade_proxy":       output.L1CrossTradeProxy,
		"l2_to_l2_cross_trade_proxy": output.L2toL2CrossTradeProxy,
		"l2_to_l2_l1_proxy":          output.L2toL2CrossTradeL1,
	}
	logger.Info("CrossTrade AWS contracts",
		zap.String("stackId", stackId.String()),
		zap.Any("contracts", contracts))

	finalMetadata := map[string]interface{}{
		"l2l1Url":   output.L2L1DAppURL,
		"l2l2Url":   output.L2L2DAppURL,
		"contracts": contracts,
	}
	metadataBytes, err := json.Marshal(finalMetadata)
	if err != nil {
		logger.Error("failed to marshal final metadata",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	if err := b.integrationRepo.UpdateMetadataAfterInstalled(
		pendingIntegration.ID.String(),
		entities.IntegrationInfo(metadataBytes),
	); err != nil {
		logger.Error("failed to update integration metadata",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	if stack.Metadata == nil {
		stack.Metadata = &entities.StackMetadata{}
	}
	stack.Metadata.L2L1CrossTradeUrl = output.L2L1DAppURL
	stack.Metadata.L2L2CrossTradeUrl = output.L2L2DAppURL
	if err := b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata after cross trade AWS",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
	}

	logger.Info("Auto-install CrossTrade AWS completed successfully",
		zap.String("stackId", stackId.String()))
}
