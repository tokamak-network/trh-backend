package presets

// Definition holds all configuration for a single preset.
type Definition struct {
	ID                 string
	Name               string
	Description        string
	Modules            map[string]bool
	GenesisPredeploys  []string
	EstimatedTime      map[string]string
	ChainDefaults      map[string]any
	HelmValues         map[string]any
	OverridableFields  []string
	AvailableFeeTokens []string
}
