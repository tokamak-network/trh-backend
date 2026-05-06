package dtos

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSanitizeBlockExplorerConfig_StripsDBCredentials verifies that DB
// username/password never appear in the sanitized response, even after JSON
// marshaling — the GET endpoint must not leak credentials.
func TestSanitizeBlockExplorerConfig_StripsDBCredentials(t *testing.T) {
	stored := &InstallBlockExplorerRequest{
		DatabaseUsername:     "blockscout",
		DatabasePassword:     "secret-password-12345",
		CoinmarketcapKey:     "cmc-key-abc",
		CoinmarketcapTokenID: "ton-station",
		WalletConnectID:      "wc-project-xyz",
	}

	resp := SanitizeBlockExplorerConfig(stored, "http://block-explorer.example.com")

	bytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	out := string(bytes)

	if strings.Contains(out, "secret-password-12345") {
		t.Errorf("response leaked DatabasePassword: %s", out)
	}
	if strings.Contains(out, "blockscout") {
		t.Errorf("response leaked DatabaseUsername: %s", out)
	}
	if strings.Contains(out, "databaseUsername") || strings.Contains(out, "databasePassword") {
		t.Errorf("response contains DB field keys: %s", out)
	}

	// Confirm safe fields ARE present
	if resp.CoinmarketcapKey != "cmc-key-abc" {
		t.Errorf("CoinmarketcapKey: got %q, want %q", resp.CoinmarketcapKey, "cmc-key-abc")
	}
	if resp.CoinmarketcapTokenID != "ton-station" {
		t.Errorf("CoinmarketcapTokenID: got %q, want %q", resp.CoinmarketcapTokenID, "ton-station")
	}
	if resp.WalletConnectID != "wc-project-xyz" {
		t.Errorf("WalletConnectID: got %q, want %q", resp.WalletConnectID, "wc-project-xyz")
	}
	if resp.URL != "http://block-explorer.example.com" {
		t.Errorf("URL: got %q, want %q", resp.URL, "http://block-explorer.example.com")
	}
}

// TestSanitizeBlockExplorerConfig_NilStored verifies nil stored input returns
// an empty response with only URL populated (defensive — should not panic).
func TestSanitizeBlockExplorerConfig_NilStored(t *testing.T) {
	resp := SanitizeBlockExplorerConfig(nil, "http://example.com")
	if resp.CoinmarketcapKey != "" || resp.CoinmarketcapTokenID != "" || resp.WalletConnectID != "" {
		t.Errorf("nil stored should produce empty CMC/WC fields, got %+v", resp)
	}
	if resp.URL != "http://example.com" {
		t.Errorf("URL should still be set, got %q", resp.URL)
	}
}
