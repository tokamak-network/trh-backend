package enum

type IntegrationType string

const (
	IntegrationTypeRegisterCandidate   IntegrationType = "register-candidate"
	IntegrationTypeRegisterMetadataDAO IntegrationType = "register-metadata-dao"
	IntegrationTypeBridge              IntegrationType = "bridge"
	IntegrationTypeCrossTradeL2ToL1    IntegrationType = "cross-trade-l2-to-l1"
	IntegrationTypeCrossTradeL2ToL2    IntegrationType = "cross-trade-l2-to-l2"
	IntegrationTypeBlockExplorer       IntegrationType = "block-explorer"
	IntegrationTypeMonitoring          IntegrationType = "monitoring"
	IntegrationTypeUptimeService       IntegrationType = "system-pulse"
)

func (i IntegrationType) String() string {
	return string(i)
}
