package entities

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StackMetadata struct {
	Layer1           string `json:"layer1"`
	Layer2           string `json:"layer2"`
	L2RpcUrl         string `json:"l2RpcUrl"`
	BridgeUrl        string `json:"bridgeUrl,omitempty"`
	L1ChainId        int    `json:"l1ChainId"`
	L2ChainId        int    `json:"l2ChainId"`
	GrafanaUrl       string `json:"grafanaUrl,omitempty"`
	ExplorerUrl      string `json:"explorerUrl,omitempty"`
	CrossTradeUrl    string `json:"crossTradeUrl,omitempty"`
	RollupConfigUrl  string `json:"rollupConfigUrl,omitempty"`
	MonitoringUrl    string `json:"monitoringUrl,omitempty"`
	UptimeServiceUrl string `json:"uptimeServiceUrl,omitempty"`
}

func (m *StackMetadata) Marshal() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func FromJSONToStackMetadata(data json.RawMessage) (*StackMetadata, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var metadata StackMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

type StackEntity struct {
	ID                  uuid.UUID         `json:"id"`
	Name                string            `json:"name"`
	Type                string            `json:"type"`
	Network             DeploymentNetwork `json:"network"`
	Target              DeploymentTarget  `json:"target"`
	Config              json.RawMessage   `json:"config"`
	DeploymentPath      string            `json:"deployment_path"`
	Metadata            *StackMetadata    `json:"metadata"`
	Status              StackStatus       `json:"status"`
	ImmutableConfig     json.RawMessage   `json:"immutable_config"`
	MainnetConfirmation json.RawMessage   `json:"mainnet_confirmation"`
	KubeconfigPath      string            `json:"kubeconfig_path"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	DeletedAt           gorm.DeletedAt    `json:"deleted_at"`
}

// ResolveTarget returns the deployment target, deriving from Network for backward compatibility
// when Target is not explicitly set (old stacks created before the Target field existed).
func (s *StackEntity) ResolveTarget() DeploymentTarget {
	if s.Target != "" {
		return s.Target
	}
	if s.Network == DeploymentNetworkLocalTestnet {
		return DeploymentTargetLocal
	}
	return DeploymentTargetCloud
}
