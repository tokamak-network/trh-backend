package enum

type IntegrationType string

const (
	IntegrationTypeRegisterCandidate   IntegrationType = "register-candidate"
	IntegrationTypeRegisterMetadataDAO IntegrationType = "register-metadata-dao"
	IntegrationTypeBridge              IntegrationType = "bridge"
	IntegrationTypeBlockExplorer       IntegrationType = "block-explorer"
	IntegrationTypeMonitoring          IntegrationType = "monitoring"
)

func (i IntegrationType) String() string {
	return string(i)
}
