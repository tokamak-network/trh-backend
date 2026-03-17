package thanos

import (
	"strings"
	"testing"
)

// Test mnemonic from BIP39 test vectors (publicly known, safe to use in tests)
const testMnemonic = "test test test test test test test test test test test junk"

func TestDeriveRoleAccounts_ValidMnemonic(t *testing.T) {
	admin, sequencer, batcher, proposer, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := map[string]string{
		"admin":     admin,
		"sequencer": sequencer,
		"batcher":   batcher,
		"proposer":  proposer,
	}

	for role, key := range keys {
		if len(key) != 64 {
			t.Errorf("role %s: expected 64-char hex key, got %d chars", role, len(key))
		}
		if strings.HasPrefix(key, "0x") {
			t.Errorf("role %s: key should not have 0x prefix", role)
		}
	}

	// All 4 keys must be distinct
	seen := make(map[string]bool)
	for role, key := range keys {
		if seen[key] {
			t.Errorf("role %s: key is a duplicate of another role key", role)
		}
		seen[key] = true
	}
}

func TestDeriveRoleAccounts_Deterministic(t *testing.T) {
	admin1, seq1, bat1, prop1, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	admin2, seq2, bat2, prop2, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if admin1 != admin2 || seq1 != seq2 || bat1 != bat2 || prop1 != prop2 {
		t.Error("key derivation is not deterministic for the same mnemonic")
	}
}

func TestDeriveRoleAccounts_InvalidMnemonic(t *testing.T) {
	_, _, _, _, err := DeriveRoleAccounts("not a valid mnemonic phrase at all foo bar baz")
	if err == nil {
		t.Error("expected error for invalid mnemonic, got nil")
	}
}

func TestDeriveRoleAccounts_DifferentMnemonics(t *testing.T) {
	mnemonic2 := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	admin1, _, _, _, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("first mnemonic failed: %v", err)
	}

	admin2, _, _, _, err := DeriveRoleAccounts(mnemonic2)
	if err != nil {
		t.Fatalf("second mnemonic failed: %v", err)
	}

	if admin1 == admin2 {
		t.Error("different mnemonics produced the same admin key")
	}
}
