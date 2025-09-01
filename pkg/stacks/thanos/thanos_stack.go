package thanos

import (
	"context"

	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	trhSDKLogging "github.com/tokamak-network/trh-sdk/pkg/logging"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/types"
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
