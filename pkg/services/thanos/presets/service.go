package presets

import "fmt"

// opPredeploys contains the base OP Stack predeploy contracts included in every preset.
// Addresses follow the 0x4200000000000000000000000000000000000xxx convention.
// EntryPoint and Paymaster are included in all presets to support non-TON fee tokens.
var opPredeploys = []string{
	"L2ToL1MessagePasser",              // 0x4200000000000000000000000000000000000016
	"L2CrossDomainMessenger",           // 0x4200000000000000000000000000000000000007
	"L2StandardBridge",                 // 0x4200000000000000000000000000000000000010
	"L2ERC721Bridge",                   // 0x4200000000000000000000000000000000000014
	"OptimismMintableERC20Factory",     // 0x4200000000000000000000000000000000000012
	"OptimismMintableERC721Factory",    // 0x4200000000000000000000000000000000000017
	"L1Block",                          // 0x4200000000000000000000000000000000000015
	"GasPriceOracle",                   // 0x420000000000000000000000000000000000000F
	"SequencerFeeVault",                // 0x4200000000000000000000000000000000000011
	"BaseFeeVault",                     // 0x4200000000000000000000000000000000000019
	"L1FeeVault",                       // 0x420000000000000000000000000000000000001A
	"SchemaRegistry",                   // 0x4200000000000000000000000000000000000020
	"EAS",                              // 0x4200000000000000000000000000000000000021
	"EntryPoint",                       // 0x4200000000000000000000000000000000000210 (ERC-4337 AA)
	"Paymaster",                        // 0x4200000000000000000000000000000000000211
}

// defiPredeploys adds DeFi-specific contracts on top of opPredeploys.
// EntryPoint and Paymaster are inherited via opPredeploys.
var defiPredeploys = append(opPredeploys,
	"UniswapV3Factory",                    // 0x4200000000000000000000000000000000000100
	"UniswapV3SwapRouter",                 // 0x4200000000000000000000000000000000000101
	"UniswapV3NonfungiblePositionManager", // 0x4200000000000000000000000000000000000102
	"USDCBridge",                          // 0x4200000000000000000000000000000000000110
	"WrappedETH",                          // 0x4200000000000000000000000000000000000111
)

// gamingPredeploys adds gaming-specific contracts on top of opPredeploys.
// EntryPoint and Paymaster are inherited via opPredeploys.
var gamingPredeploys = append(opPredeploys,
	"VRF",            // 0x4200000000000000000000000000000000000200
	"VRFCoordinator", // 0x4200000000000000000000000000000000000201
)

// fullPredeploys combines DeFi and Gaming additions on top of opPredeploys.
// EntryPoint and Paymaster are inherited via opPredeploys.
var fullPredeploys = append(opPredeploys,
	"UniswapV3Factory",
	"UniswapV3SwapRouter",
	"UniswapV3NonfungiblePositionManager",
	"USDCBridge",
	"WrappedETH",
	"VRF",
	"VRFCoordinator",
)

// DefaultPresetDefinitions holds all backend-owned preset definitions.
var DefaultPresetDefinitions = map[string]Definition{
	"general": {
		ID:          "general",
		Name:        "General Purpose",
		Description: "Baseline rollup preset for standard application workloads.",
		Modules: map[string]bool{
			"bridge":        true,
			"blockExplorer": true,
			"monitoring":    false,
			"crossTrade":    false,
			"uptimeService": false,
		},
		GenesisPredeploys: opPredeploys,
		EstimatedTime: map[string]string{
			"deploy":      "20-30m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 1800,
			"outputRootFrequency":      1800,
			"challengePeriod":          12,
			"registerCandidate":        false,
			"backupEnabled":            false,
		},
		HelmValues: map[string]any{
			"bridge.enabled":        true,
			"monitoring.enabled":    false,
			"blockscout.enabled":    true,
			"crossTrade.enabled":    false,
			"uptimeService.enabled": false,
		},
		OverridableFields: []string{
			"l2BlockTime",
			"batchSubmissionFrequency",
			"outputRootFrequency",
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
			"crossTrade":    true,
			"uptimeService": true,
		},
		GenesisPredeploys: defiPredeploys,
		EstimatedTime: map[string]string{
			"deploy":      "30-40m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 900,
			"outputRootFrequency":      900,
			"challengePeriod":          12,
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
			"backupEnabled",
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
			"crossTrade":    false,
			"uptimeService": true,
			"drb":           true,
		},
		GenesisPredeploys: gamingPredeploys,
		EstimatedTime: map[string]string{
			"deploy":      "35-45m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 300,
			"outputRootFrequency":      600,
			"challengePeriod":          12,
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
			"l2BlockTime",
			"batchSubmissionFrequency",
			"outputRootFrequency",
			"backupEnabled",
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
			"drb":           true,
		},
		GenesisPredeploys: fullPredeploys,
		EstimatedTime: map[string]string{
			"deploy":      "40-50m",
			"fundingWait": "5-15m",
		},
		ChainDefaults: map[string]any{
			"l2BlockTime":              2,
			"batchSubmissionFrequency": 600,
			"outputRootFrequency":      600,
			"challengePeriod":          12,
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
			"backupEnabled",
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
