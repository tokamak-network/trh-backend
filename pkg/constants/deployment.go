package constants

const (
	DeployL1ContractsStep         = "deploy-l1-contracts"
	DeployAWSInfraStep            = "deploy-aws-infra"
	DeployLocalInfraStep          = "deploy-local-infra"
	DestroyChainStep              = "destroy-chain"
	InstallBlockExplorerStep      = "install-block-explorer"
	UninstallBlockExplorerStep    = "uninstall-block-explorer"
	InstallBridgeStep             = "install-bridge"
	UninstallBridgeStep           = "uninstall-bridge"
	InstallMonitoringStep         = "install-monitoring"
	UninstallMonitoringStep       = "uninstall-monitoring"
	RegisterCandidateStep         = "register-candidate"
	RegisterMetadataDAOStep       = "register-metadata-dao"
	InstallCrossTradeBridgeStep   = "install-cross-trade-bridge"
	UninstallCrossTradeBridgeStep = "uninstall-cross-trade-bridge"
	InstallUptimeServiceStep      = "install-system-pulse"
	UninstallUptimeServiceStep    = "uninstall-system-pulse"
	UninstallDRBStep              = "uninstall-drb"
	InstallCrossTradeL2L1Step     = "install-cross-trade-l2-l1"
	UninstallCrossTradeL2L1Step   = "uninstall-cross-trade-l2-l1"
	InstallCrossTradeL2L2Step     = "install-cross-trade-l2-l2"
	UninstallCrossTradeL2L2Step   = "uninstall-cross-trade-l2-l2"
	RegisterTokensL2L1Step        = "register-tokens-l2-l1"
	RegisterTokensL2L2Step        = "register-tokens-l2-l2"
	DeployNewL2ChainL2L1Step      = "deploy-new-l2-chain-l2-l1"
	DeployNewL2ChainL2L2Step      = "deploy-new-l2-chain-l2-l2"
	InstallDRBStep                = "install-drb"
)

// GetDeployInfraStepName returns the appropriate infra deployment step name based on provider
func GetDeployInfraStepName(infraProvider string) string {
	if infraProvider == "local" {
		return DeployLocalInfraStep
	}
	return DeployAWSInfraStep
}
