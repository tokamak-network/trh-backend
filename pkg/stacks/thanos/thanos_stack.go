package thanos

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-sdk/pkg/constants"
	thanosConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
	trhSDKLogging "github.com/tokamak-network/trh-sdk/pkg/logging"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/types"
	"go.uber.org/zap"
)

const (
	DeployL1CrossTradeL2L1 = "DeployL1CrossTrade_L2L1.s.sol"
	DeployL2CrossTradeL2L1 = "DeployL2CrossTrade_L2L1.s.sol"
	DeployL1CrossTradeL2L2 = "DeployL1CrossTrade_L2L2.s.sol"
	DeployL2CrossTradeL2L2 = "DeployL2CrossTrade_L2L2.s.sol"
)

const (
	L2L2CrossTradeProxyL1ContractName = "L2toL2CrossTradeProxyL1"
	L2L2CrossTradeL1ContractName      = "L2toL2CrossTradeL1"
	L1L2CrossTradeProxyL1ContractName = "L1CrossTradeProxy"
	L1L2CrossTradeL1ContractName      = "L1CrossTrade"

	L2L2CrossTradeProxyL2ContractName = "L2toL2CrossTradeProxy"
	L2L2CrossTradeL2ContractName      = "L2toL2CrossTradeL2"
	L1L2CrossTradeProxyL2ContractName = "L2CrossTradeProxy"
	L1L2CrossTradeL2ContractName      = "L2CrossTrade"
)

const (
	L2L2ScriptPath = "scripts/foundry_scripts"
	L1L2ScriptPath = "scripts/foundry_scripts/L2L1"
)

func NewThanosSDKClient(
	ctx context.Context,
	logPath string,
	network string,
	deploymentPath string,
	registerCandidate bool,
	awsAccessKey string,
	awsSecretAccessKey string,
	awsRegion string,
) (*thanosStack.ThanosStack, error) {
	l, err := trhSDKLogging.InitLogger(logPath)
	if err != nil {
		return nil, err
	}

	logger.Info("Initializing Thanos SDK...")

	var awsConfig *thanosTypes.AWSConfig

	if awsAccessKey != "" && awsSecretAccessKey != "" && awsRegion != "" {
		awsConfig = &thanosTypes.AWSConfig{
			AccessKey: awsAccessKey,
			SecretKey: awsSecretAccessKey,
			Region:    awsRegion,
		}
	}

	s, err := thanosStack.NewThanosStack(ctx, l, network, false, deploymentPath, awsConfig)
	if err != nil {
		logger.Errorf("Failed to create thanos stacks: %s", err)
		return nil, err
	}

	if registerCandidate {
		s.SetRegisterCandidate(true)
	}

	return s, nil
}

func DeployAWSInfrastructure(ctx context.Context, sdkClient *thanosStack.ThanosStack, req *dtos.DeployThanosAWSInfraRequest) error {
	logger.Info("Deploying AWS Infrastructure...")

	deployInfraInput := thanosStack.DeployInfraInput{
		ChainName:   req.ChainName,
		L1BeaconURL: req.L1BeaconUrl,
		Mnemonic:    req.SeedPhrase,
	}
	if req.BackupConfig != nil {
		deployInfraInput.BackupConfig = &thanosStack.BackupConfig{
			Enabled: req.BackupConfig.Enabled,
		}
	} else {
		deployInfraInput.BackupConfig = &thanosStack.BackupConfig{
			Enabled: false,
		}
	}

	err := sdkClient.Deploy(ctx, consts.AWS, &deployInfraInput)
	if err != nil {
		return err
	}

	logger.Info("AWS Infrastructure deployed successfully")

	return nil
}

func DeployLocalInfrastructure(ctx context.Context, sdkClient *thanosStack.ThanosStack, req *dtos.DeployThanosAWSInfraRequest) error {
	logger.Info("Deploying Local Infrastructure (Docker Compose)...")

	backupEnabled := false
	if req.BackupConfig != nil {
		backupEnabled = req.BackupConfig.Enabled
	}

	deployInfraInput := &thanosStack.DeployInfraInput{
		ChainName:   req.ChainName,
		L1BeaconURL: req.L1BeaconUrl,
		Mnemonic:    req.SeedPhrase,
		BackupConfig: &thanosStack.BackupConfig{
			Enabled: backupEnabled,
		},
	}

	err := sdkClient.Deploy(ctx, consts.Local, deployInfraInput)
	if err != nil {
		return err
	}
	logger.Info("Local Infrastructure deployed successfully")
	return nil
}

// StartAAOperatorFromConfig runs the AA operator goroutine using the SDK client's loaded
// deployment config (FeeToken, AdminPrivateKey). Blocks until ctx is cancelled.
// Intended to be called via taskManager.AddTask so it runs as a managed background task.
func StartAAOperatorFromConfig(ctx context.Context, sdkClient *thanosStack.ThanosStack) {
	sdkClient.RunAAOperatorFromConfig(ctx)
}

func DestroyAWSInfrastructure(ctx context.Context, sdkClient *thanosStack.ThanosStack) error {
	logger.Info("Destroying AWS Infrastructure...")

	err := sdkClient.Destroy(ctx)
	if err != nil {
		return err
	}

	logger.Info("AWS Infrastructure destroyed successfully")

	return nil
}

func DeployL1Contracts(ctx context.Context, sdkClient *thanosStack.ThanosStack, req *dtos.DeployL1ContractsRequest) error {
	logger.Info("Deploying L1 Contracts...")

	chainConfig := thanosTypes.ChainConfiguration{
		BatchSubmissionFrequency: uint64(req.BatchSubmissionFrequency),
		ChallengePeriod:          uint64(req.ChallengePeriod),
		OutputRootFrequency:      uint64(req.OutputRootFrequency),
		L2BlockTime:              uint64(req.L2BlockTime),
		L1BlockTime:              12,
	}
	operators := thanosTypes.Operators{
		AdminPrivateKey:      req.AdminAccount,
		SequencerPrivateKey:  req.SequencerAccount,
		BatcherPrivateKey:    req.BatcherAccount,
		ProposerPrivateKey:   req.ProposerAccount,
		ChallengerPrivateKey: req.ChallengerAccount,
	}

	contractDeploymentInput := thanosStack.DeployContractsInput{
		L1RPCurl:           req.L1RpcUrl,
		ChainConfiguration: &chainConfig,
		Operators:          &operators,
		ReuseDeployment:    req.ReuseDeployment,
		EnableFaultProof:   req.EnableFaultProof,
		Preset:             req.Preset,
		FeeToken:           req.FeeToken,
		Mnemonic:           req.SeedPhrase,
	}

	if req.RegisterCandidate && req.RegisterCandidateParams != nil {
		contractDeploymentInput.RegisterCandidate = &thanosStack.RegisterCandidateInput{
			Amount:   req.RegisterCandidateParams.Amount,
			Memo:     req.RegisterCandidateParams.Memo,
			NameInfo: req.RegisterCandidateParams.NameInfo,
			UseTon:   true, // TODO: we only support TON for now
		}
	}

	err := sdkClient.DeployContracts(ctx, &contractDeploymentInput)
	if err != nil {
		return err
	}

	logger.Info("L1 Contracts deployed successfully")
	return nil
}

func ShowChainInformation(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) (*thanosTypes.ChainInformation, error) {
	return sdkClient.ShowInformation(ctx)
}

// BuildLocalChainInformation constructs chain information for local infra deployments
// where K8s is not available. URLs use localhost ports defined in the local compose template.
func BuildLocalChainInformation(deploymentPath string) *thanosTypes.ChainInformation {
	info := &thanosTypes.ChainInformation{
		L2RpcUrl:       "http://localhost:8545",
		BridgeUrl:      "http://localhost:3001",
		BlockExplorer:  "http://localhost:4001",
		MonitoringUrl:  "http://localhost:3002",
		RollupFilePath: deploymentPath + "/rollup.json",
	}

	rollupPath := deploymentPath + "/rollup.json"
	if data, err := os.ReadFile(rollupPath); err == nil {
		var rollup struct {
			L1ChainID int `json:"l1_chain_id"`
			L2ChainID int `json:"l2_chain_id"`
		}
		if err := json.Unmarshal(data, &rollup); err == nil {
			info.L1ChainID = rollup.L1ChainID
			info.L2ChainID = rollup.L2ChainID
		}
	}

	return info
}

func InstallBridge(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) (string, error) {
	return sdkClient.InstallBridge(ctx)
}

func UninstallBridge(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) error {
	return sdkClient.UninstallBridge(ctx)
}

func InstallBlockExplorer(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	req *dtos.InstallBlockExplorerRequest,
) (string, error) {
	return s.InstallBlockExplorer(ctx, &thanosStack.InstallBlockExplorerInput{
		DatabaseUsername:       req.DatabaseUsername,
		DatabasePassword:       req.DatabasePassword,
		CoinmarketcapKey:       req.CoinmarketcapKey,
		WalletConnectProjectID: req.WalletConnectID,
	})
}

func UninstallBlockExplorer(
	ctx context.Context,
	s *thanosStack.ThanosStack,
) error {
	return s.UninstallBlockExplorer(ctx)
}

func GetMonitoringConfig(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	req *dtos.InstallMonitoringRequest,
) (*thanosTypes.MonitoringConfig, error) {
	alertManager := req.AlertManager
	password := req.GrafanaPassword
	loggingEnabled := req.LoggingEnabled
	telegramReceivers := make([]thanosTypes.TelegramReceiver, len(alertManager.Telegram.CriticalReceivers))
	for i, receiver := range alertManager.Telegram.CriticalReceivers {
		telegramReceivers[i] = thanosTypes.TelegramReceiver{
			ChatId: receiver.ChatId,
		}
	}

	thanosAlertManagerConfig := thanosTypes.AlertManagerConfig{
		Telegram: thanosTypes.TelegramConfig{
			Enabled:           alertManager.Telegram.Enabled,
			ApiToken:          alertManager.Telegram.ApiToken,
			CriticalReceivers: telegramReceivers,
		},
		Email: thanosTypes.EmailConfig{
			Enabled:          alertManager.Email.Enabled,
			SmtpSmarthost:    alertManager.Email.SmtpSmarthost,
			SmtpFrom:         alertManager.Email.SmtpFrom,
			SmtpAuthPassword: alertManager.Email.SmtpAuthPassword,
			AlertReceivers:   alertManager.Email.AlertReceivers,
		},
	}
	return s.GetMonitoringConfig(ctx, password, thanosAlertManagerConfig, loggingEnabled)
}

func InstallMonitoring(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	config *thanosTypes.MonitoringConfig,
) (*thanosTypes.MonitoringInfo, error) {
	return s.InstallMonitoring(ctx, config)
}

func UninstallMonitoring(
	ctx context.Context,
	s *thanosStack.ThanosStack,
) error {
	return s.UninstallMonitoring(ctx)
}

func UpdateNetwork(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	req *dtos.UpdateNetworkRequest,
) error {
	return s.UpdateNetwork(ctx, &thanosStack.UpdateNetworkInput{
		L1RPC:       req.L1RpcUrl,
		L1BeaconURL: req.L1BeaconUrl,
	})
}

func VerifyRegisterCandidates(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	req *dtos.RegisterCandidateRequest,
) error {
	return s.VerifyRegisterCandidates(ctx, &thanosStack.RegisterCandidateInput{
		Amount:   req.Amount,
		Memo:     req.Memo,
		NameInfo: req.NameInfo,
		UseTon:   true, // TODO: we only support TON for now
	})
}

func GetRegisterCandidatesInfo(ctx context.Context, s *thanosStack.ThanosStack, registerCandidate *dtos.RegisterCandidateRequest) (*thanosTypes.RegistrationAdditionalInfo, error) {
	return s.GetRegistrationAdditionalInfo(ctx, &thanosStack.RegisterCandidateInput{
		Amount:   registerCandidate.Amount,
		Memo:     registerCandidate.Memo,
		NameInfo: registerCandidate.NameInfo,
		UseTon:   true, // TODO: we only support TON for now
	})
}

func RegisterMetadataDAO(ctx context.Context, s *thanosStack.ThanosStack, registerMetadataDAO *dtos.RegisterMetadataDAORequest) (*thanosTypes.RegisterMetadataDaoResult, error) {
	chainInfo := thanosTypes.ChainInfo{
		Description: registerMetadataDAO.Metadata.Chain.Description,
		Logo:        registerMetadataDAO.Metadata.Chain.Logo,
		Website:     registerMetadataDAO.Metadata.Chain.Website,
	}
	bridgeInfo := thanosTypes.BridgeInfo{
		Name: registerMetadataDAO.Metadata.Bridge.Name,
	}
	explorerInfo := thanosTypes.ExplorerInfo{
		Name: registerMetadataDAO.Metadata.Explorer.Name,
	}
	supportResources := thanosTypes.SupportResources{
		StatusPageUrl:     registerMetadataDAO.Metadata.Support.StatusPageUrl,
		SupportContactUrl: registerMetadataDAO.Metadata.Support.SupportContactUrl,
		DocumentationUrl:  registerMetadataDAO.Metadata.Support.DocumentationUrl,
		CommunityUrl:      registerMetadataDAO.Metadata.Support.CommunityUrl,
		HelpCenterUrl:     registerMetadataDAO.Metadata.Support.HelpCenterUrl,
		AnnouncementUrl:   registerMetadataDAO.Metadata.Support.AnnouncementUrl,
	}
	metadataInfo := thanosTypes.MetadataInfo{
		Chain:    chainInfo,
		Bridge:   bridgeInfo,
		Explorer: explorerInfo,
		Support:  supportResources,
	}
	return s.RegisterMetadata(ctx, &thanosTypes.GitHubCredentials{
		Username: registerMetadataDAO.Username,
		Token:    registerMetadataDAO.Token,
		Email:    registerMetadataDAO.Email,
	}, &metadataInfo)
}

func BackupSnapshot(ctx context.Context, s *thanosStack.ThanosStack) (*thanosTypes.BackupSnapshotInfo, error) {
	snapshotInfo, err := s.BackupSnapshot(ctx, nil)
	if err != nil {
		return nil, err
	}
	return snapshotInfo, nil
}

func GetListBackup(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.BackupRequest) ([]thanosTypes.RecoveryPoint, error) {
	backupListInfo, err := s.BackupList(ctx, req.Limit)
	if err != nil {
		return nil, err
	}
	return backupListInfo.RecoveryPoints, nil
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

// RemoveEmailConfig removes email configuration from AlertManager
func RemoveEmailConfig(ctx context.Context, s *thanosStack.ThanosStack) error {
	alertCustomization := &thanosStack.AlertCustomization{Stack: s}
	return alertCustomization.RemoveEmailConfig(ctx)
}

// RemoveTelegramConfig removes telegram configuration from AlertManager
func RemoveTelegramConfig(ctx context.Context, s *thanosStack.ThanosStack) error {
	alertCustomization := &thanosStack.AlertCustomization{Stack: s}
	return alertCustomization.RemoveTelegramConfig(ctx)
}

// UpdateEmailConfig updates email configuration in AlertManager
func UpdateEmailConfig(ctx context.Context, s *thanosStack.ThanosStack, emailConfig *dtos.UpdateEmailConfigRequest) error {
	alertCustomization := &thanosStack.AlertCustomization{Stack: s}

	// For Gmail, smtpUsername should be the same as smtpFrom (email address)
	return alertCustomization.UpdateEmailConfig(ctx,
		emailConfig.SmtpSmarthost,
		emailConfig.SmtpFrom,
		emailConfig.SmtpFrom,         // smtpUsername = smtpFrom (email address)
		emailConfig.SmtpAuthPassword, // smtpPassword
		emailConfig.AlertReceivers,
	)
}

// UpdateTelegramConfig updates telegram configuration in AlertManager
func UpdateTelegramConfig(ctx context.Context, s *thanosStack.ThanosStack, telegramConfig *dtos.UpdateTelegramConfigRequest) error {
	alertCustomization := &thanosStack.AlertCustomization{Stack: s}

	// Convert receivers to comma-separated string
	var receiversStr string
	if len(telegramConfig.CriticalReceivers) > 0 {
		chatIds := make([]string, len(telegramConfig.CriticalReceivers))
		for i, receiver := range telegramConfig.CriticalReceivers {
			chatIds[i] = receiver.ChatId
		}
		receiversStr = strings.Join(chatIds, ",")
	}

	return alertCustomization.UpdateTelegramConfig(ctx, telegramConfig.ApiToken, receiversStr)
}

func InstallCrossTradeBridge(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.InstallCrossChainBridgeRequest) (*thanosTypes.DeployCrossTradeOutput, error) {
	var blockExplorerConfig *thanosTypes.BlockExplorerConfig
	if req.L1ChainConfig.BlockExplorerConfig != nil {
		blockExplorerConfig = &thanosTypes.BlockExplorerConfig{
			APIKey: req.L1ChainConfig.BlockExplorerConfig.APIKey,
			URL:    req.L1ChainConfig.BlockExplorerConfig.URL,
			Type:   req.L1ChainConfig.BlockExplorerConfig.Type,
		}
	}

	if !strings.HasPrefix(req.L1ChainConfig.PrivateKey, "0x") {
		req.L1ChainConfig.PrivateKey = "0x" + req.L1ChainConfig.PrivateKey
	}

	l1ChainConfig := &thanosTypes.L1CrossTradeChainInput{
		RPC:                    req.L1ChainConfig.RPC,
		ChainID:                req.L1ChainConfig.ChainID,
		PrivateKey:             req.L1ChainConfig.PrivateKey,
		IsDeployedNew:          req.L1ChainConfig.IsDeployedNew,
		BlockExplorerConfig:    blockExplorerConfig,
		CrossTradeProxyAddress: req.L1ChainConfig.CrossTradeProxyAddress,
		CrossTradeAddress:      req.L1ChainConfig.CrossTradeAddress,
		ChainName:              req.L1ChainConfig.ChainName,
	}

	l2ChainConfigs := make([]*thanosTypes.L2CrossTradeChainInput, len(req.L2ChainConfig))
	for i, chainConfig := range req.L2ChainConfig {
		var blockExplorerAPIKey, blockExplorerURL string
		var blockExplorerType constants.BlockExplorerType
		if chainConfig.BlockExplorerConfig != nil {
			blockExplorerAPIKey = chainConfig.BlockExplorerConfig.APIKey
			blockExplorerURL = chainConfig.BlockExplorerConfig.URL
			blockExplorerType = chainConfig.BlockExplorerConfig.Type
		}

		crossDomainMessenger := chainConfig.CrossDomainMessenger
		if crossDomainMessenger == "" {
			crossDomainMessenger = "0x4200000000000000000000000000000000000007"
		}

		if !strings.HasPrefix(chainConfig.PrivateKey, "0x") {
			chainConfig.PrivateKey = "0x" + chainConfig.PrivateKey
		}

		l2ChainConfigs[i] = &thanosTypes.L2CrossTradeChainInput{
			RPC:           chainConfig.RPC,
			ChainID:       chainConfig.ChainID,
			PrivateKey:    chainConfig.PrivateKey,
			IsDeployedNew: chainConfig.IsDeployedNew,
			ChainName:     chainConfig.ChainName,
			BlockExplorerConfig: &thanosTypes.BlockExplorerConfig{
				APIKey: blockExplorerAPIKey,
				URL:    blockExplorerURL,
				Type:   blockExplorerType,
			},
			CrossDomainMessenger:    crossDomainMessenger,
			CrossTradeProxyAddress:  chainConfig.CrossTradeProxyAddress,
			CrossTradeAddress:       chainConfig.CrossTradeAddress,
			NativeTokenAddressOnL1:  chainConfig.NativeTokenAddress,
			L1StandardBridgeAddress: chainConfig.L1StandardBridgeAddress,
			L1USDCBridgeAddress:     chainConfig.L1USDCBridgeAddress,
			L1CrossDomainMessenger:  chainConfig.L1CrossDomainMessenger,
		}
	}

	logger.Info("l1 chain config", zap.Any("l1 chain config", l1ChainConfig))
	logger.Info("l2 chain configs", zap.Any("l2 chain configs", l2ChainConfigs))

	output, err := s.DeployCrossTradeContracts(ctx, &thanosTypes.CrossTrade{
		ProjectID:     req.ProjectID,
		Mode:          req.Mode,
		L1ChainConfig: l1ChainConfig,
		L2ChainConfig: l2ChainConfigs,
	}, true)
	if err != nil {
		logger.Error("failed to deploy cross trade", zap.Error(err))
		return nil, err
	}

	return output, nil
}

func UninstallCrossTradeBridge(ctx context.Context, s *thanosStack.ThanosStack, mode string) error {
	return s.UninstallCrossTrade(ctx, constants.CrossTradeDeployMode(mode))
}

// DeployCrossTradeLocal deploys CrossTrade contracts on a local Docker Compose L2
// via L1 OptimismPortal depositTransaction calls (Go-native, no Foundry).
// This is the local-infra path; the AWS path uses InstallCrossTradeBridge.
// l1CrossTradeProxy and l2ToL2CrossTradeL1 are the pre-deployed L1 contract addresses.
// supportedTokens may be empty for Phase 1 (no token pairs registered on deploy).
func DeployCrossTradeLocal(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
	deployerPrivKey string,
	l1RpcUrl string,
	l2RpcUrl string,
	l1ChainID uint64,
	l2ChainID uint64,
	optimismPortalProxy string,
	crossDomainMessenger string,
	l1CrossTradeProxy string,
	l2ToL2CrossTradeL1 string,
	supportedTokens []thanosStack.TokenPair,
) (*thanosStack.DeployCrossTradeLocalOutput, error) {
	return sdkClient.DeployCrossTradeLocal(ctx, &thanosStack.DeployCrossTradeLocalInput{
		L1RPCUrl:             l1RpcUrl,
		L1ChainID:            l1ChainID,
		DeployerPrivateKey:   deployerPrivKey,
		L2RPCUrl:             l2RpcUrl,
		L2ChainID:            l2ChainID,
		OptimismPortalProxy:  optimismPortalProxy,
		CrossDomainMessenger: crossDomainMessenger,
		L1CrossTradeProxy:    l1CrossTradeProxy,
		L2toL2CrossTradeL1:   l2ToL2CrossTradeL1,
		SupportedTokens:      supportedTokens,
	})
}

func InstallUptimeService(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.InstallUptimeServiceRequest) (string, error) {
	config, err := s.GetUptimeServiceConfig(ctx)
	if err != nil {
		return "", err
	}
	return s.InstallUptimeService(ctx, config)
}

func UninstallUptimeService(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) error {
	return sdkClient.UninstallUptimeService(ctx)
}

func UninstallDRB(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) error {
	return sdkClient.UninstallDRB(ctx)
}


func GetDefaultContractAddresses() map[uint64]thanosConstants.DefaultContractAddress {
	return thanosConstants.DefaultContractAddresses
}

func GetDeployedL2ChainConfigurationForCrossTrade(ctx context.Context, s *thanosStack.ThanosStack, mode constants.CrossTradeDeployMode) ([]*thanosTypes.L2CrossTradeChainInput, error) {
	crossTradeInfo, err := s.GetCrossTradeConfiguration(ctx, mode)
	if err != nil {
		return nil, err
	}
	return crossTradeInfo.L2ChainConfig, nil
}

func RegisterTokens(ctx context.Context, s *thanosStack.ThanosStack, mode constants.CrossTradeDeployMode, req []*dtos.RegisterTokensRequest) (*thanosTypes.DeployCrossTradeOutput, error) {
	tokenConfigs := make([]*thanosTypes.RegisterTokenInput, 0)
	for _, tokenConfig := range req {
		l2TokenInputs := make([]*thanosTypes.L2TokenInput, 0)
		for _, l2TokenInput := range tokenConfig.L2TokenInputs {
			l2TokenInputs = append(l2TokenInputs, &thanosTypes.L2TokenInput{
				ChainID:      l2TokenInput.ChainID,
				TokenAddress: l2TokenInput.TokenAddress,
			})
		}
		tokenConfigs = append(tokenConfigs, &thanosTypes.RegisterTokenInput{
			TokenName:      tokenConfig.TokenName,
			L1TokenAddress: tokenConfig.L1TokenAddress,
			L2TokenInputs:  l2TokenInputs,
		})
	}
	output, err := s.RegisterNewTokensOnExistingCrossTrade(ctx, mode, tokenConfigs)
	if err != nil {
		logger.Error("failed to register tokens", zap.Error(err))
		return nil, err
	}
	return output, nil
}

func DeployNewL2Chain(ctx context.Context, s *thanosStack.ThanosStack, mode constants.CrossTradeDeployMode, req *dtos.L2CrossTradeChainInput) (*thanosTypes.DeployCrossTradeOutput, error) {
	crossTradeInfo, err := s.GetCrossTradeConfiguration(ctx, mode)
	if err != nil {
		logger.Error("failed to get deployed l2 chain configuration for cross trade", zap.Error(err))
		return nil, err
	}

	// Reset the current l2 chains on the config and add the new l2 chain
	var blockExplorerConfig *thanosTypes.BlockExplorerConfig
	if req.BlockExplorerConfig != nil {
		blockExplorerConfig = &thanosTypes.BlockExplorerConfig{
			APIKey: req.BlockExplorerConfig.APIKey,
			URL:    req.BlockExplorerConfig.URL,
			Type:   req.BlockExplorerConfig.Type,
		}
	}
	l2ChainConfigs := []*thanosTypes.L2CrossTradeChainInput{
		{
			RPC:                     req.RPC,
			ChainID:                 req.ChainID,
			PrivateKey:              req.PrivateKey,
			IsDeployedNew:           req.IsDeployedNew,
			ChainName:               req.ChainName,
			BlockExplorerConfig:     blockExplorerConfig,
			CrossDomainMessenger:    req.CrossDomainMessenger,
			CrossTradeProxyAddress:  req.CrossTradeProxyAddress,
			CrossTradeAddress:       req.CrossTradeAddress,
			NativeTokenAddressOnL1:  req.NativeTokenAddress,
			L1StandardBridgeAddress: req.L1StandardBridgeAddress,
			L1USDCBridgeAddress:     req.L1USDCBridgeAddress,
			L1CrossDomainMessenger:  req.L1CrossDomainMessenger,
		},
	}

	// Read for the deployment file
	l1ChainConfig := &thanosTypes.L1CrossTradeChainInput{
		RPC:                    crossTradeInfo.L1ChainConfig.RPC,
		ChainID:                crossTradeInfo.L1ChainConfig.ChainID,
		PrivateKey:             crossTradeInfo.L1ChainConfig.PrivateKey,
		IsDeployedNew:          false,
		CrossTradeProxyAddress: crossTradeInfo.L1ChainConfig.CrossTradeProxyAddress,
		CrossTradeAddress:      crossTradeInfo.L1ChainConfig.CrossTradeAddress,
		DeploymentScriptPath:   crossTradeInfo.L1ChainConfig.DeploymentScriptPath,
		BlockExplorerConfig:    crossTradeInfo.L1ChainConfig.BlockExplorerConfig,
		ChainName:              crossTradeInfo.L1ChainConfig.ChainName,
	}

	params := &thanosTypes.CrossTrade{
		ProjectID:     crossTradeInfo.ProjectID,
		Mode:          constants.CrossTradeDeployMode(mode),
		L1ChainConfig: l1ChainConfig,
		L2ChainConfig: l2ChainConfigs,
	}
	output, err := s.DeployCrossTradeContracts(ctx, params, false)
	if err != nil {
		logger.Error("failed to deploy new l2 chain", zap.Error(err))
		return nil, err
	}
	return output, nil
}
