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
)

// GetDeployInfraStepName returns the appropriate infra deployment step name based on provider
func GetDeployInfraStepName(infraProvider string) string {
	if infraProvider == "local" {
		return DeployLocalInfraStep
	}
	return DeployAWSInfraStep
}
