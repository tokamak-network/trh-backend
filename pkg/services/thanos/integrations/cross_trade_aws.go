package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
	trhSDKUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
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

	l2l1Req := BuildDefaultCrossTradeL2L1Request(
		stackConfig.L1RpcUrl,
		l1ChainID,
		l2RPC,
		l2ChainID,
		stackConfig.AdminAccount,
		stackId.String(),
	)

	var awsCfg CrossTradeAWSConfig

	logPath := fmt.Sprintf("/tmp/cross-trade-aws-%s-l2l1.log", stackId.String())
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
		logger.Error("failed to create SDK client for L2_TO_L1",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	l2l1Output, err := thanos.InstallCrossTradeBridge(ctx, sdkClient, l2l1Req)
	if err != nil {
		logger.Error("failed to install CrossTrade L2_TO_L1",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	logger.Info("CrossTrade L2_TO_L1 installed successfully",
		zap.String("stackId", stackId.String()))

	awsCfg.L2ToL1 = l2l1Req
	if cfgBytes, cfgErr := json.Marshal(awsCfg); cfgErr == nil {
		if updateErr := b.integrationRepo.UpdateConfig(pendingIntegration.ID.String(), json.RawMessage(cfgBytes)); updateErr != nil {
			logger.Error("failed to save L2_TO_L1 config",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(pendingIntegration.ID.String(), entities.DeploymentStatusFailed, updateErr.Error())
			return
		}
	}

	if l2l1Output != nil && l2l1Output.DeployCrossTradeApplicationOutput.URL != "" {
		if stack.Metadata == nil {
			stack.Metadata = &entities.StackMetadata{}
		}
		stack.Metadata.L2L1CrossTradeUrl = l2l1Output.DeployCrossTradeApplicationOutput.URL
		if err := b.stackRepo.UpdateMetadata(stackId.String(), stack.Metadata); err != nil {
			logger.Error("failed to update stack metadata after L2_TO_L1",
				zap.String("stackId", stackId.String()),
				zap.Error(err))
			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(pendingIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error())
			return
		}
	}

	l1Contracts, err := trhSDKUtils.ReadDeployementConfigFromJSONFile(stack.DeploymentPath, l1ChainID)
	if err != nil {
		logger.Error("failed to read L1 contracts for CrossTrade L2_TO_L2",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			fmt.Sprintf("read L1 contracts for L2_TO_L2: %v", err),
		)
		return
	}

	l2l2Req := BuildDefaultCrossTradeL2L2Request(
		stackConfig.L1RpcUrl,
		l1ChainID,
		l2RPC,
		l2ChainID,
		l1Contracts.L1StandardBridgeProxy,
		l1Contracts.L1UsdcBridgeProxy,
		l1Contracts.L1CrossDomainMessengerProxy,
		stackConfig.AdminAccount,
		stackId.String(),
	)

	logPath = fmt.Sprintf("/tmp/cross-trade-aws-%s-l2l2.log", stackId.String())
	sdkClient, err = thanos.NewThanosSDKClient(
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
		logger.Error("failed to create SDK client for L2_TO_L2",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	l2l2Output, err := thanos.InstallCrossTradeBridge(ctx, sdkClient, l2l2Req)
	if err != nil {
		logger.Error("failed to install CrossTrade L2_TO_L2",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			pendingIntegration.ID.String(),
			entities.DeploymentStatusFailed,
			err.Error(),
		)
		return
	}

	logger.Info("CrossTrade L2_TO_L2 installed successfully",
		zap.String("stackId", stackId.String()))

	awsCfg.L2ToL2 = l2l2Req
	if cfgBytes, cfgErr := json.Marshal(awsCfg); cfgErr == nil {
		_ = b.integrationRepo.UpdateConfig(pendingIntegration.ID.String(), json.RawMessage(cfgBytes))
	}

	l2l1URL := ""
	if l2l1Output != nil {
		l2l1URL = l2l1Output.DeployCrossTradeApplicationOutput.URL
	}
	l2l2URL := ""
	if l2l2Output != nil {
		l2l2URL = l2l2Output.DeployCrossTradeApplicationOutput.URL
	}

	contracts := map[string]string{}
	if l2l1Output != nil && l2l1Output.DeployCrossTradeContractsOutput != nil {
		if addr := l2l1Output.DeployCrossTradeContractsOutput.L2CrossTradeProxyAddresses[l2ChainID]; addr != "" {
			contracts["l2_cross_trade_proxy"] = addr
		}
		if addr := l2l1Output.DeployCrossTradeContractsOutput.L1CrossTradeProxyAddress; addr != "" {
			contracts["l1_cross_trade_proxy"] = addr
		}
	}
	if l2l2Output != nil && l2l2Output.DeployCrossTradeContractsOutput != nil {
		if addr := l2l2Output.DeployCrossTradeContractsOutput.L2CrossTradeProxyAddresses[l2ChainID]; addr != "" {
			contracts["l2_to_l2_cross_trade_proxy"] = addr
		}
		if addr := l2l2Output.DeployCrossTradeContractsOutput.L1CrossTradeProxyAddress; addr != "" {
			contracts["l2_to_l2_l1_proxy"] = addr
		}
	}
	logger.Info("CrossTrade AWS contracts extracted",
		zap.String("stackId", stackId.String()),
		zap.Any("contracts", contracts))

	finalMetadata := map[string]interface{}{
		"l2l1Url":   l2l1URL,
		"l2l2Url":   l2l2URL,
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
	stack.Metadata.L2L1CrossTradeUrl = l2l1URL
	stack.Metadata.L2L2CrossTradeUrl = l2l2URL
	if err := b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata after cross trade AWS",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
	}

	logger.Info("Auto-install CrossTrade AWS completed successfully",
		zap.String("stackId", stackId.String()))
}
