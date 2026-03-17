package presets

// Definition holds all configuration for a single preset.
type Definition struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Modules            map[string]bool   `json:"modules"`
	GenesisPredeploys  []string          `json:"genesisPredeploys"`
	EstimatedTime      map[string]string `json:"estimatedTime"`
	ChainDefaults      map[string]any    `json:"chainDefaults"`
	HelmValues         map[string]any    `json:"helmValues"`
	OverridableFields  []string          `json:"overridableFields"`
	AvailableFeeTokens []string          `json:"availableFeeTokens"`
}
