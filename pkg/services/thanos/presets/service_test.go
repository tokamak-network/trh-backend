package presets_test

import (
	"testing"

	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
)

func TestListAll_ReturnsFourPresets(t *testing.T) {
	svc := presets.NewService()
	defs := svc.ListAll()

	if len(defs) != 4 {
		t.Fatalf("expected 4 presets, got %d", len(defs))
	}
}

func TestListAll_OrderIsStable(t *testing.T) {
	svc := presets.NewService()
	want := []string{"general", "defi", "gaming", "full"}

	defs := svc.ListAll()
	for i, id := range want {
		if defs[i].ID != id {
			t.Errorf("position %d: expected %q, got %q", i, id, defs[i].ID)
		}
	}
}

func TestGetByID_KnownPresets(t *testing.T) {
	svc := presets.NewService()
	ids := []string{"general", "defi", "gaming", "full"}

	for _, id := range ids {
		def, err := svc.GetByID(id)
		if err != nil {
			t.Errorf("GetByID(%q) returned unexpected error: %v", id, err)
		}
		if def == nil {
			t.Errorf("GetByID(%q) returned nil definition", id)
			continue
		}
		if def.ID != id {
			t.Errorf("GetByID(%q): definition ID mismatch, got %q", id, def.ID)
		}
		if def.Name == "" {
			t.Errorf("GetByID(%q): Name is empty", id)
		}
		if def.Description == "" {
			t.Errorf("GetByID(%q): Description is empty", id)
		}
	}
}

func TestGetByID_UnknownPreset_ReturnsError(t *testing.T) {
	svc := presets.NewService()

	_, err := svc.GetByID("unknown")
	if err == nil {
		t.Fatal("expected error for unknown preset ID, got nil")
	}
}

func TestGetByID_EmptyID_ReturnsError(t *testing.T) {
	svc := presets.NewService()

	_, err := svc.GetByID("")
	if err == nil {
		t.Fatal("expected error for empty preset ID, got nil")
	}
}

func TestPresetDefinitions_ModulesNotEmpty(t *testing.T) {
	svc := presets.NewService()
	defs := svc.ListAll()

	for _, def := range defs {
		if len(def.Modules) == 0 {
			t.Errorf("preset %q: Modules is empty", def.ID)
		}
	}
}

func TestPresetDefinitions_ChainDefaultsHaveRequiredKeys(t *testing.T) {
	required := []string{
		"l2BlockTime",
		"batchSubmissionFrequency",
		"outputRootFrequency",
		"challengePeriod",
	}

	svc := presets.NewService()
	for _, def := range svc.ListAll() {
		for _, key := range required {
			if _, ok := def.ChainDefaults[key]; !ok {
				t.Errorf("preset %q: ChainDefaults missing key %q", def.ID, key)
			}
		}
	}
}

func TestPresetDefinitions_BridgeAlwaysEnabled(t *testing.T) {
	svc := presets.NewService()

	for _, def := range svc.ListAll() {
		if !def.Modules["bridge"] {
			t.Errorf("preset %q: bridge module should always be enabled", def.ID)
		}
	}
}

func TestPresetDefinitions_OverridableFieldsNotEmpty(t *testing.T) {
	svc := presets.NewService()

	for _, def := range svc.ListAll() {
		if len(def.OverridableFields) == 0 {
			t.Errorf("preset %q: OverridableFields is empty", def.ID)
		}
	}
}

func TestPresetDefinitions_GenesisPredeploys(t *testing.T) {
	expectedPredeploys := []string{
		"L2StandardBridge",
		"L2CrossDomainMessenger",
		"OptimismMintableERC20Factory",
	}

	svc := presets.NewService()
	for _, def := range svc.ListAll() {
		if len(def.GenesisPredeploys) != len(expectedPredeploys) {
			t.Errorf("preset %q: expected %d genesis predeploys, got %d",
				def.ID, len(expectedPredeploys), len(def.GenesisPredeploys))
			continue
		}
		for i, p := range expectedPredeploys {
			if def.GenesisPredeploys[i] != p {
				t.Errorf("preset %q: predeploy[%d] expected %q, got %q",
					def.ID, i, p, def.GenesisPredeploys[i])
			}
		}
	}
}

func TestPresetDefinitions_EstimatedTimeHasDeployAndFundingWait(t *testing.T) {
	svc := presets.NewService()

	for _, def := range svc.ListAll() {
		if def.EstimatedTime["deploy"] == "" {
			t.Errorf("preset %q: EstimatedTime[deploy] is empty", def.ID)
		}
		if def.EstimatedTime["fundingWait"] == "" {
			t.Errorf("preset %q: EstimatedTime[fundingWait] is empty", def.ID)
		}
	}
}
