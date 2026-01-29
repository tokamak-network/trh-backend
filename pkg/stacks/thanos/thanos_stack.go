package thanos

import (
	"context"
	"strings"

	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	trhSDKLogging "github.com/tokamak-network/trh-sdk/pkg/logging"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/types"
	"go.uber.org/zap"
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
		ChallengerPrivateKey: "", // TODO: enable challenger in the future when we support fp
	}

	contractDeploymentInput := thanosStack.DeployContractsInput{
		L1RPCurl:           req.L1RpcUrl,
		ChainConfiguration: &chainConfig,
		Operators:          &operators,
	}

	if req.RegisterCandidate {
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

func BackupRestore(ctx context.Context, s *thanosStack.ThanosStack, request dtos.BackupRestoreRequest) (*thanosTypes.BackupRestoreInfo, error) {
	backupRestoreInfo, err := s.BackupRestore(ctx, request.RecoveryPointID, request.Attach, request.Pvcs, request.Stss, nil)
	if err != nil {
		return nil, err
	}
	return backupRestoreInfo, nil
}

func BackupAttach(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.BackupAttachRequest) (*thanosTypes.BackupAttachInfo, error) {
	backupAttachInfo, err := s.BackupAttach(ctx, req.EfsId, req.Pvcs, req.Stss)
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

func InstallCrossTradeBridge(ctx context.Context, s *thanosStack.ThanosStack, req *dtos.InstallCrossChainBridgeRequest) (*thanosStack.DeployCrossTradeOutput, error) {
	l2ChainConfigs := make([]*thanosStack.L2CrossTradeChainInput, len(req.L2ChainConfig))
	for i, chainConfig := range req.L2ChainConfig {
		l2ChainConfigs[i] = &thanosStack.L2CrossTradeChainInput{
			RPC:                  chainConfig.RPC,
			ChainID:              chainConfig.ChainID,
			PrivateKey:           chainConfig.PrivateKey,
			IsDeployedNew:        chainConfig.IsDeployedNew,
			DeploymentScriptPath: chainConfig.DeploymentScriptPath,
			ContractName:         chainConfig.ContractName,
			BlockExplorerConfig: &thanosStack.BlockExplorerConfig{
				APIKey: chainConfig.BlockExplorerConfig.APIKey,
				URL:    chainConfig.BlockExplorerConfig.URL,
				Type:   chainConfig.BlockExplorerConfig.Type,
			},
			CrossDomainMessenger:   chainConfig.CrossDomainMessenger,
			CrossTradeProxyAddress: chainConfig.CrossTradeProxyAddress,
			CrossTradeAddress:      chainConfig.CrossTradeAddress,
			USDTAddress:            chainConfig.USDTAddress,
			USDCAddress:            chainConfig.USDCAddress,
			TONAddress:             chainConfig.TONAddress,
		}
	}
	l1ChainConfig := &thanosStack.L1CrossTradeChainInput{
		RPC:                  req.L1ChainConfig.RPC,
		ChainID:              req.L1ChainConfig.ChainID,
		PrivateKey:           req.L1ChainConfig.PrivateKey,
		IsDeployedNew:        req.L1ChainConfig.IsDeployedNew,
		DeploymentScriptPath: req.L1ChainConfig.DeploymentScriptPath,
		ContractName:         req.L1ChainConfig.ContractName,
	}

	logger.Info("l1 chain config", zap.Any("l1 chain config", l1ChainConfig))
	logger.Info("l2 chain configs", zap.Any("l2 chain configs", l2ChainConfigs))

	return s.DeployCrossTrade(ctx, &thanosStack.DeployCrossTradeInputs{
		ProjectID:     req.ProjectID,
		Mode:          req.Mode,
		L1ChainConfig: l1ChainConfig,
		L2ChainConfig: l2ChainConfigs,
	})
}

func UninstallCrossTradeBridge(ctx context.Context, s *thanosStack.ThanosStack) error {
	return s.UninstallCrossTrade(ctx)
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
