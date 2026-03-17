package presets

import "fmt"

// DefaultPresetDefinitions holds all backend-owned preset definitions.
var DefaultPresetDefinitions = map[string]Definition{
	"general": {
		ID:          "general",
		Name:        "General Purpose",
		Description: "Baseline rollup preset for standard application workloads.",
		Modules: map[string]bool{
			"bridge":        true,
			"blockExplorer": false,
			"monitoring":    false,
			"crossTrade":    false,
			"uptimeService": false,
		},
		GenesisPredeploys: []string{
			"L2StandardBridge",
			"L2CrossDomainMessenger",
			"OptimismMintableERC20Factory",
		},
		EstimatedTime: map[string]string{
			"deploy":      "20-30m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 1800,
			"outputRootFrequency":      1800,
			"challengePeriod":          86400,
			"registerCandidate":        false,
			"backupEnabled":            false,
		},
		HelmValues: map[string]any{
			"bridge.enabled":        true,
			"monitoring.enabled":    false,
			"blockscout.enabled":    false,
			"crossTrade.enabled":    false,
			"uptimeService.enabled": false,
		},
		OverridableFields: []string{
			"l2BlockTime",
			"batchSubmissionFrequency",
			"outputRootFrequency",
			"challengePeriod",
			"backupEnabled",
		},
		AvailableFeeTokens: []string{"TON", "ETH", "USDT", "USDC"},
	},
	"defi": {
		ID:          "defi",
		Name:        "DeFi",
		Description: "Preset for exchange, liquidity, and settlement-heavy workloads.",
		Modules: map[string]bool{
			"bridge":        true,
			"blockExplorer": true,
			"monitoring":    true,
			"crossTrade":    false,
			"uptimeService": true,
		},
		GenesisPredeploys: []string{
			"L2StandardBridge",
			"L2CrossDomainMessenger",
			"OptimismMintableERC20Factory",
		},
		EstimatedTime: map[string]string{
			"deploy":      "30-40m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 900,
			"outputRootFrequency":      900,
			"challengePeriod":          86400,
			"registerCandidate":        false,
			"backupEnabled":            true,
		},
		HelmValues: map[string]any{
			"bridge.enabled":        true,
			"monitoring.enabled":    true,
			"blockscout.enabled":    true,
			"crossTrade.enabled":    false,
			"uptimeService.enabled": true,
		},
		OverridableFields: []string{
			"batchSubmissionFrequency",
			"outputRootFrequency",
			"challengePeriod",
		},
		AvailableFeeTokens: []string{"TON", "ETH", "USDT", "USDC"},
	},
	"gaming": {
		ID:          "gaming",
		Name:        "Gaming",
		Description: "Preset optimized for higher throughput and player-facing observability.",
		Modules: map[string]bool{
			"bridge":        true,
			"blockExplorer": true,
			"monitoring":    true,
			"crossTrade":    true,
			"uptimeService": true,
		},
		GenesisPredeploys: []string{
			"L2StandardBridge",
			"L2CrossDomainMessenger",
			"OptimismMintableERC20Factory",
		},
		EstimatedTime: map[string]string{
			"deploy":      "35-45m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 300,
			"outputRootFrequency":      600,
			"challengePeriod":          86400,
			"registerCandidate":        false,
			"backupEnabled":            true,
		},
		HelmValues: map[string]any{
			"bridge.enabled":        true,
			"monitoring.enabled":    true,
			"blockscout.enabled":    true,
			"crossTrade.enabled":    true,
			"uptimeService.enabled": true,
		},
		OverridableFields: []string{
			"l2BlockTime",
			"batchSubmissionFrequency",
			"outputRootFrequency",
		},
		AvailableFeeTokens: []string{"TON", "ETH", "USDT", "USDC"},
	},
	"full": {
		ID:          "full",
		Name:        "Full Suite",
		Description: "All recommended modules enabled for demos, staging, or high-touch managed environments.",
		Modules: map[string]bool{
			"bridge":        true,
			"blockExplorer": true,
			"monitoring":    true,
			"crossTrade":    true,
			"uptimeService": true,
		},
		GenesisPredeploys: []string{
			"L2StandardBridge",
			"L2CrossDomainMessenger",
			"OptimismMintableERC20Factory",
		},
		EstimatedTime: map[string]string{
			"deploy":      "40-50m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 600,
			"outputRootFrequency":      600,
			"challengePeriod":          86400,
			"registerCandidate":        true,
			"backupEnabled":            true,
		},
		HelmValues: map[string]any{
			"bridge.enabled":        true,
			"monitoring.enabled":    true,
			"blockscout.enabled":    true,
			"crossTrade.enabled":    true,
			"uptimeService.enabled": true,
		},
		OverridableFields: []string{
			"l2BlockTime",
			"batchSubmissionFrequency",
			"outputRootFrequency",
			"challengePeriod",
			"registerCandidate",
		},
		AvailableFeeTokens: []string{"TON", "ETH", "USDT", "USDC"},
	},
}

// Service provides preset discovery operations.
type Service struct{}

// NewService creates a new preset Service.
func NewService() *Service {
	return &Service{}
}

// ListAll returns all preset definitions in insertion-stable order.
func (s *Service) ListAll() []Definition {
	order := []string{"general", "defi", "gaming", "full"}
	result := make([]Definition, 0, len(order))
	for _, id := range order {
		if def, ok := DefaultPresetDefinitions[id]; ok {
			result = append(result, def)
		}
	}
	return result
}

// GetByID returns a single preset definition.
// Returns an error when the preset ID is unknown.
func (s *Service) GetByID(id string) (*Definition, error) {
	def, ok := DefaultPresetDefinitions[id]
	if !ok {
		return nil, fmt.Errorf("unknown preset id: %q", id)
	}
	return &def, nil
}
