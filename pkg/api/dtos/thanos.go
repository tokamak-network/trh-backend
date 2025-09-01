package dtos

import (
	"context"
	"errors"
	"net/mail"
	"regexp"

	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	trhSdkAws "github.com/tokamak-network/trh-sdk/pkg/cloud-provider/aws"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	trhSdkTypes "github.com/tokamak-network/trh-sdk/pkg/types"
	trhSdkUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"go.uber.org/zap"
)

var chainNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9 ]*$`)

type RegisterCandidateRequest struct {
	Amount   float64 `json:"amount" binding:"required" validate:"min=0"`
	Memo     string  `json:"memo" binding:"required"`
	NameInfo string  `json:"nameInfo"`
}

func (r *RegisterCandidateRequest) Validate(ctx context.Context) error {
	registerCandidateParams := thanosStack.RegisterCandidateInput{
		Amount:   r.Amount,
		Memo:     r.Memo,
		NameInfo: r.NameInfo,
		UseTon:   true,
	}

	if err := registerCandidateParams.Validate(ctx); err != nil {
		return err
	}

	return nil
}

type DeployThanosRequest struct {
	Network                  entities.DeploymentNetwork `json:"network"                  binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	L1RpcUrl                 string                     `json:"l1RpcUrl"                 binding:"required" validate:"url"`
	L1BeaconUrl              string                     `json:"l1BeaconUrl"              binding:"required" validate:"url"`
	L2BlockTime              int                        `json:"l2BlockTime"              binding:"required" validate:"min=1"` // seconds
	BatchSubmissionFrequency int                        `json:"batchSubmissionFrequency" binding:"required" validate:"min=1"` // seconds
	OutputRootFrequency      int                        `json:"outputRootFrequency"      binding:"required" validate:"min=1"` // seconds
	ChallengePeriod          int                        `json:"challengePeriod"          binding:"required" validate:"min=1"` // seconds
	AdminAccount             string                     `json:"adminAccount"             binding:"required" validate:"eth_address"`
	SequencerAccount         string                     `json:"sequencerAccount"         binding:"required" validate:"eth_address"`
	BatcherAccount           string                     `json:"batcherAccount"           binding:"required" validate:"eth_address"`
	ProposerAccount          string                     `json:"proposerAccount"          binding:"required" validate:"eth_address"`
	AwsAccessKey             string                     `json:"awsAccessKey"             binding:"required"`
	AwsSecretAccessKey       string                     `json:"awsSecretAccessKey"       binding:"required"`
	AwsRegion                string                     `json:"awsRegion"                binding:"required"`
	ChainName                string                     `json:"chainName"                binding:"required"`
	DeploymentPath           string                     `json:"deploymentPath"`
	RegisterCandidate        bool                       `json:"registerCandidate"`
	RegisterCandidateParams  *RegisterCandidateRequest  `json:"registerCandidateParams,omitempty"`
}

func (request *DeployThanosRequest) Validate() error {
	if request.Network == entities.DeploymentNetworkLocalDevnet {
		return errors.New("local devnet is not supported yet")
	}

	// Validate Chain Name
	if !chainNameRegex.MatchString(request.ChainName) {
		logger.Error("invalid chainName", zap.String("chainName", request.ChainName))
		return errors.New(
			"invalid chain name, chain name must contain only letters (a-z, A-Z), numbers (0-9), spaces. Special characters are not allowed",
		)
	}

	// Validate L1 RPC URL
	if !trhSdkUtils.IsValidL1RPC(request.L1RpcUrl) {
		logger.Error("invalid l1RpcUrl", zap.String("l1RpcUrl", request.L1RpcUrl))
		return errors.New("invalid l1RpcUrl")
	}

	// Validate L1 Beacon URL
	if !trhSdkUtils.IsValidBeaconURL(request.L1BeaconUrl) {
		logger.Error("invalid l1BeaconUrl", zap.String("l1BeaconUrl", request.L1BeaconUrl))
		return errors.New("invalid l1BeaconUrl")
	}

	// Validate AWS Access Key
	if !trhSdkUtils.IsValidAWSAccessKey(request.AwsAccessKey) {
		logger.Error("invalid awsAccessKey", zap.String("awsAccessKey", request.AwsAccessKey))
		return errors.New("invalid awsAccessKey")
	}

	// Validate AWS Secret Key
	if !trhSdkUtils.IsValidAWSSecretKey(request.AwsSecretAccessKey) {
		logger.Error(
			"invalid awsSecretKey",
			zap.String("awsSecretAccessKey", request.AwsSecretAccessKey),
		)
		return errors.New("invalid awsSecretKey")
	}

	// Validate AWS Region
	if !trhSdkAws.IsAvailableRegion(
		request.AwsAccessKey,
		request.AwsSecretAccessKey,
		request.AwsRegion,
	) {
		logger.Error("invalid awsRegion", zap.String("awsRegion", request.AwsRegion))
		return errors.New("invalid awsRegion")
	}

	// Validate Chain Config
	chainID, err := utils.GetChainIDFromRPC(request.L1RpcUrl)
	if err != nil {
		logger.Error("invalid rpc", zap.String("chainId", err.Error()))
		return errors.New("invalid rpc")
	}
	chainConfig := trhSdkTypes.ChainConfiguration{
		BatchSubmissionFrequency: uint64(request.BatchSubmissionFrequency),
		OutputRootFrequency:      uint64(request.OutputRootFrequency),
		ChallengePeriod:          uint64(request.ChallengePeriod),
		L2BlockTime:              uint64(request.L2BlockTime),
		L1BlockTime:              consts.L1_BLOCK_TIME,
	}

	err = chainConfig.Validate(chainID)
	if err != nil {
		logger.Error("invalid chainConfig", zap.String("chainConfig", err.Error()))
		return err
	}

	return nil
}

type DeployL1ContractsRequest struct {
	L1RpcUrl                 string                    `json:"l1RpcUrl"                 binding:"required" validate:"url"`
	L2BlockTime              int                       `json:"l2BlockTime"              binding:"required" validate:"min=1"` // seconds
	BatchSubmissionFrequency int                       `json:"batchSubmissionFrequency" binding:"required" validate:"min=1"` // seconds
	OutputRootFrequency      int                       `json:"outputRootFrequency"      binding:"required" validate:"min=1"` // seconds
	ChallengePeriod          int                       `json:"challengePeriod"          binding:"required" validate:"min=1"` // seconds
	AdminAccount             string                    `json:"adminAccount"             binding:"required" validate:"eth_address"`
	SequencerAccount         string                    `json:"sequencerAccount"         binding:"required" validate:"eth_address"`
	BatcherAccount           string                    `json:"batcherAccount"           binding:"required" validate:"eth_address"`
	ProposerAccount          string                    `json:"proposerAccount"          binding:"required" validate:"eth_address"`
	RegisterCandidate        bool                      `json:"registerCandidate"`
	RegisterCandidateParams  *RegisterCandidateRequest `json:"registerCandidateParams,omitempty"`
}

type DeployThanosAWSInfraRequest struct {
	ChainName   string `json:"chainName"          binding:"required"`
	L1BeaconUrl string `json:"l1BeaconUrl"        binding:"required" validate:"url"`
}

type InstallBlockExplorerRequest struct {
	DatabaseUsername string `json:"databaseUsername"     binding:"required"`
	DatabasePassword string `json:"databasePassword"     binding:"required"`
	CoinmarketcapKey string `json:"coinmarketcapKey"     binding:"required"`
	WalletConnectID  string `json:"walletConnectId"     binding:"required"`
}

func (r *InstallBlockExplorerRequest) Validate() error {
	if err := trhSdkUtils.ValidatePostgresUsername(r.DatabaseUsername); err != nil {
		logger.Error("invalid database username", zap.String("databaseUsername", r.DatabaseUsername))
		return errors.New("invalid database username")
	}

	if !trhSdkUtils.IsValidRDSUsername(r.DatabaseUsername) {
		logger.Error("invalid database username", zap.String("databaseUsername", r.DatabaseUsername))
		return errors.New("invalid database username")
	}

	if !trhSdkUtils.IsValidRDSPassword(r.DatabasePassword) {
		logger.Error("invalid database password", zap.String("databasePassword", r.DatabasePassword))
		return errors.New("invalid database password")
	}

	if r.CoinmarketcapKey == "" {
		logger.Error("coinmarketcapKey is required")
		return errors.New("coinmarketcapKey is required")
	}
	if r.WalletConnectID == "" {
		logger.Error("walletConnectId is required")
		return errors.New("walletConnectId is required")
	}

	return nil
}

type TerminateThanosRequest struct {
	Network            string `json:"network"            binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	AwsAccessKey       string `json:"awsAccessKey"       binding:"required"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey" binding:"required"`
	AwsRegion          string `json:"awsRegion"          binding:"required"`
	DeploymentPath     string `json:"deploymentPath"     binding:"required"`
	LogPath            string `json:"logPath"            binding:"required"`
}

type DeployThanosResponse struct {
	Id string `json:"id"`
}

type InstallPluginsRequest struct {
	Plugins []string `json:"plugins"`
}

func (r *InstallPluginsRequest) Validate() error {
	if len(r.Plugins) == 0 {
		return errors.New("no plugins")
	}

	return nil
}

type UpdateNetworkRequest struct {
	L1RpcUrl    string `json:"l1RpcUrl" validate:"url"`
	L1BeaconUrl string `json:"l1BeaconUrl" validate:"url"`
}

// TelegramConfig holds Telegram notification configuration
type TelegramConfig struct {
	Enabled           bool               `json:"enabled"`
	ApiToken          string             `json:"apiToken"`
	CriticalReceivers []TelegramReceiver `json:"criticalReceivers"`
}

// TelegramReceiver represents a Telegram chat recipient
type TelegramReceiver struct {
	ChatId string
}

// EmailConfig holds email notification configuration
type EmailConfig struct {
	Enabled          bool     `json:"enabled"`
	SmtpSmarthost    string   `json:"smtpSmarthost"`
	SmtpFrom         string   `json:"smtpFrom"`
	SmtpAuthPassword string   `json:"smtpAuthPassword"`
	AlertReceivers   []string `json:"alertReceivers"`
}

type AlertManagerConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	Email    EmailConfig    `json:"email"`
}

type InstallMonitoringRequest struct {
	GrafanaPassword string             `json:"grafanaPassword" binding:"required"`
	AlertManager    AlertManagerConfig `json:"alertManager" binding:"required"`
	LoggingEnabled  bool               `json:"loggingEnabled"`
}

func (r *InstallMonitoringRequest) Validate() error {
	telegramReceivers := make([]trhSdkTypes.TelegramReceiver, len(r.AlertManager.Telegram.CriticalReceivers))
	for i, receiver := range r.AlertManager.Telegram.CriticalReceivers {
		telegramReceivers[i] = trhSdkTypes.TelegramReceiver{
			ChatId: receiver.ChatId,
		}
	}
	installMonitoringInputs := thanosStack.InstallMonitoringInput{
		AdminPassword: r.GrafanaPassword,
		AlertManager: trhSdkTypes.AlertManagerConfig{
			Telegram: trhSdkTypes.TelegramConfig{
				Enabled:           r.AlertManager.Telegram.Enabled,
				ApiToken:          r.AlertManager.Telegram.ApiToken,
				CriticalReceivers: telegramReceivers,
			},
			Email: trhSdkTypes.EmailConfig{
				Enabled:          r.AlertManager.Email.Enabled,
				SmtpSmarthost:    r.AlertManager.Email.SmtpSmarthost,
				SmtpFrom:         r.AlertManager.Email.SmtpFrom,
				SmtpAuthPassword: r.AlertManager.Email.SmtpAuthPassword,
				AlertReceivers:   r.AlertManager.Email.AlertReceivers,
			},
		},
		LoggingEnabled: r.LoggingEnabled,
	}

	if err := installMonitoringInputs.Validate(); err != nil {
		return err
	}

	return nil
}

type ChainInfo struct {
	Description string `json:"description"`
	Logo        string `json:"logo"`
	Website     string `json:"website"`
}

type BridgeInfo struct {
	Name string `json:"name"`
}

type ExplorerInfo struct {
	Name string `json:"name"`
}

type SupportResources struct {
	StatusPageUrl     string `json:"statusPageUrl"`
	SupportContactUrl string `json:"supportContactUrl"`
	DocumentationUrl  string `json:"documentationUrl"`
	CommunityUrl      string `json:"communityUrl"`
	HelpCenterUrl     string `json:"helpCenterUrl"`
	AnnouncementUrl   string `json:"announcementUrl"`
}

type MetadataInfo struct {
	Chain    ChainInfo        `json:"chain"`
	Bridge   BridgeInfo       `json:"bridge"`
	Explorer ExplorerInfo     `json:"explorer"`
	Support  SupportResources `json:"supportResources"`
}

type RegisterMetadataDAORequest struct {
	Username string        `json:"username" binding:"required"`
	Token    string        `json:"token" binding:"required"`
	Email    string        `json:"email" binding:"required"`
	Metadata *MetadataInfo `json:"metadata" binding:"required"`
}

func (r *RegisterMetadataDAORequest) Validate(ctx context.Context) (*RegisterMetadataDAORequest, error) {
	if r.Username == "" {
		return nil, errors.New("username is required")
	}
	if r.Token == "" {
		return nil, errors.New("token is required")
	}
	if r.Email == "" {
		return nil, errors.New("email is required")
	}

	if _, err := mail.ParseAddress(r.Email); err != nil {
		return nil, errors.New("invalid email")
	}

	if r.Metadata == nil {
		r.Metadata = &MetadataInfo{
			Chain: ChainInfo{
				Description: "Example rollup deployed with TRH SDK",
				Logo:        "https://example.com/logo.png",
				Website:     "https://example-l2.com",
			},
			Bridge: BridgeInfo{
				Name: "Example Bridge",
			},
			Explorer: ExplorerInfo{
				Name: "Example Explorer",
			},
			Support: SupportResources{
				StatusPageUrl:     "https://status.example-l2.com",
				SupportContactUrl: "https://discord.gg/example-support",
				DocumentationUrl:  "https://docs.example-l2.com",
				CommunityUrl:      "https://t.me/example_community",
				HelpCenterUrl:     "https://help.example-l2.com",
				AnnouncementUrl:   "https://twitter.com/example_l2",
			},
		}
	}

	if r.Metadata.Chain.Description == "" {
		r.Metadata.Chain.Description = "Example rollup deployed with TRH SDK"
	}

	if r.Metadata.Chain.Logo == "" {
		r.Metadata.Chain.Logo = "https://example.com/logo.png"
	}

	if r.Metadata.Chain.Website == "" {
		r.Metadata.Chain.Website = "https://example-l2.com"
	}

	if r.Metadata.Bridge.Name == "" {
		r.Metadata.Bridge.Name = "Example Bridge"
	}

	if r.Metadata.Explorer.Name == "" {
		r.Metadata.Explorer.Name = "Example Explorer"
	}

	if r.Metadata.Support.StatusPageUrl == "" {
		r.Metadata.Support.StatusPageUrl = "https://status.example-l2.com"
	}

	if r.Metadata.Support.SupportContactUrl == "" {
		r.Metadata.Support.SupportContactUrl = "https://discord.gg/example-support"
	}

	if r.Metadata.Support.DocumentationUrl == "" {
		r.Metadata.Support.DocumentationUrl = "https://docs.example-l2.com"
	}

	if r.Metadata.Support.CommunityUrl == "" {
		r.Metadata.Support.CommunityUrl = "https://t.me/example_community"
	}

	if r.Metadata.Support.HelpCenterUrl == "" {
		r.Metadata.Support.HelpCenterUrl = "https://help.example-l2.com"
	}

	if r.Metadata.Support.AnnouncementUrl == "" {
		r.Metadata.Support.AnnouncementUrl = "https://twitter.com/example_l2"
	}

	return r, nil
}
