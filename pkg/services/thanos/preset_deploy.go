package thanos

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
)

// CreateThanosStackFromPreset creates a Thanos stack from a simplified preset-based request.
// It derives role accounts from the seed phrase, fills chain parameters from the preset
// defaults and user overrides, then delegates to CreateThanosStack.
func (s *ThanosStackDeploymentService) CreateThanosStackFromPreset(
	ctx context.Context,
	req dtos.PresetDeployRequest,
) (*entities.Response, error) {
	// 1. Load preset definition
	presetSvc := presets.NewService()
	def, err := presetSvc.GetByID(req.PresetID)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("unknown preset: %s", req.PresetID),
		}, nil
	}

	// 2. Derive 4 role accounts from seed phrase
	adminKey, sequencerKey, batcherKey, proposerKey, err := DeriveRoleAccounts(req.SeedPhrase)
	// Clear seed phrase from memory immediately after derivation
	req.SeedPhrase = ""
	if err != nil {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("failed to derive accounts from seed phrase: %s", err.Error()),
		}, nil
	}

	// 3. Extract chain parameters from preset defaults
	l2BlockTime := chainDefaultInt(def.ChainDefaults, "l2BlockTime", 2)
	batchFreq := chainDefaultInt(def.ChainDefaults, "batchSubmissionFrequency", 1800)
	outputFreq := chainDefaultInt(def.ChainDefaults, "outputRootFrequency", 1800)
	challengePeriod := chainDefaultInt(def.ChainDefaults, "challengePeriod", 10)
	registerCandidate := chainDefaultBool(def.ChainDefaults, "registerCandidate", false)
	backupEnabled := chainDefaultBool(def.ChainDefaults, "backupEnabled", false)

	// Force challenge period for mainnet (non-overridable)
	if strings.EqualFold(req.Network, "Mainnet") {
		challengePeriod = 6048000
	}

	// 4. Apply user overrides (only for fields listed in preset's OverridableFields)
	overridableSet := make(map[string]bool, len(def.OverridableFields))
	for _, f := range def.OverridableFields {
		overridableSet[f] = true
	}
	for _, override := range req.Overrides {
		if !overridableSet[override.Field] {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: fmt.Sprintf("field %q is not overridable for preset %s", override.Field, req.PresetID),
			}, nil
		}
		switch override.Field {
		case "l2BlockTime":
			if v, ok := anyToInt(override.Value); ok {
				l2BlockTime = v
			}
		case "batchSubmissionFrequency":
			if v, ok := anyToInt(override.Value); ok {
				batchFreq = v
			}
		case "outputRootFrequency":
			if v, ok := anyToInt(override.Value); ok {
				outputFreq = v
			}
		case "challengePeriod":
			if !strings.EqualFold(req.Network, "Mainnet") {
				if v, ok := anyToInt(override.Value); ok {
					challengePeriod = v
				}
			}
		case "backupEnabled":
			if b, ok := override.Value.(bool); ok {
				backupEnabled = b
			}
		case "registerCandidate":
			if b, ok := override.Value.(bool); ok {
				registerCandidate = b
			}
		}
	}

	// 5. Validate provider-specific requirements
	if err := req.ValidateProvider(); err != nil {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		}, nil
	}

	// 6. Build the full deployment request
	deployReq := dtos.DeployThanosRequest{
		Network:                  req.Network,
		L1RpcUrl:                 req.L1RpcUrl,
		L1BeaconUrl:              req.L1BeaconUrl,
		L2BlockTime:              l2BlockTime,
		BatchSubmissionFrequency: batchFreq,
		OutputRootFrequency:      outputFreq,
		ChallengePeriod:          challengePeriod,
		AdminAccount:             utils.TrimPrivateKey(adminKey),
		SequencerAccount:         utils.TrimPrivateKey(sequencerKey),
		BatcherAccount:           utils.TrimPrivateKey(batcherKey),
		ProposerAccount:          utils.TrimPrivateKey(proposerKey),
		AwsAccessKey:             req.AwsAccessKey,
		AwsSecretAccessKey:       req.AwsSecretKey,
		AwsRegion:                req.AwsRegion,
		ChainName:                req.ChainName,
		RegisterCandidate:        registerCandidate,
		PresetID:                 req.PresetID,
		FeeToken:                 strings.ToUpper(req.FeeToken),
		InfraProvider:            req.InfraProvider,
	}
	if backupEnabled {
		deployReq.BackupConfig = &dtos.BackupConfig{Enabled: true}
	}

	// 7. Validate the full deployment request (only runs AWS-specific checks when provider is aws)
	if req.InfraProvider != "local" {
		if err := deployReq.Validate(); err != nil {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			}, nil
		}
	}

	// 7. Create the stack
	resp, err := s.CreateThanosStack(ctx, deployReq)
	if err != nil {
		return resp, err
	}

	// 8. Map stackId → deploymentId to match the frontend API contract
	if resp.Status == http.StatusOK {
		if data, ok := resp.Data.(map[string]string); ok {
			if stackID, ok := data["stackId"]; ok {
				resp.Data = map[string]string{"deploymentId": stackID}
			}
		}
	}
	return resp, nil
}

// chainDefaultInt extracts an int value from preset ChainDefaults map with a fallback.
func chainDefaultInt(defaults map[string]any, key string, fallback int) int {
	v, ok := defaults[key]
	if !ok {
		return fallback
	}
	if n, ok := anyToInt(v); ok {
		return n
	}
	return fallback
}

// chainDefaultBool extracts a bool value from preset ChainDefaults map with a fallback.
func chainDefaultBool(defaults map[string]any, key string, fallback bool) bool {
	v, ok := defaults[key]
	if !ok {
		return fallback
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}

// anyToInt converts common numeric types to int.
func anyToInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	case int64:
		return int(val), true
	case int32:
		return int(val), true
	}
	return 0, false
}
