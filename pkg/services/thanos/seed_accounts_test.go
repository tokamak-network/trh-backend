package thanos

import (
	"strings"
	"testing"
)

// Test mnemonic from BIP39 test vectors (publicly known, safe to use in tests)
const testMnemonic = "test test test test test test test test test test test junk"

func TestDeriveRoleAccounts_ValidMnemonic(t *testing.T) {
	admin, sequencer, batcher, proposer, challenger, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := map[string]string{
		"admin":      admin,
		"sequencer":  sequencer,
		"batcher":    batcher,
		"proposer":   proposer,
		"challenger": challenger,
	}

	for role, key := range keys {
		if len(key) != 64 {
			t.Errorf("role %s: expected 64-char hex key, got %d chars", role, len(key))
		}
		if strings.HasPrefix(key, "0x") {
			t.Errorf("role %s: key should not have 0x prefix", role)
		}
	}

	// All 5 keys must be distinct
	seen := make(map[string]bool)
	for role, key := range keys {
		if seen[key] {
			t.Errorf("role %s: key is a duplicate of another role key", role)
		}
		seen[key] = true
	}
}

func TestDeriveRoleAccounts_Deterministic(t *testing.T) {
	admin1, seq1, bat1, prop1, chal1, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	admin2, seq2, bat2, prop2, chal2, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if admin1 != admin2 || seq1 != seq2 || bat1 != bat2 || prop1 != prop2 || chal1 != chal2 {
		t.Error("key derivation is not deterministic for the same mnemonic")
	}
}

func TestDeriveRoleAccounts_InvalidMnemonic(t *testing.T) {
	_, _, _, _, _, err := DeriveRoleAccounts("not a valid mnemonic phrase at all foo bar baz")
	if err == nil {
		t.Error("expected error for invalid mnemonic, got nil")
	}
}

func TestDeriveRoleAccounts_DifferentMnemonics(t *testing.T) {
	mnemonic2 := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	admin1, _, _, _, _, err := DeriveRoleAccounts(testMnemonic)
	if err != nil {
		t.Fatalf("first mnemonic failed: %v", err)
	}

	admin2, _, _, _, _, err := DeriveRoleAccounts(mnemonic2)
	if err != nil {
		t.Fatalf("second mnemonic failed: %v", err)
	}

	if admin1 == admin2 {
		t.Error("different mnemonics produced the same admin key")
	}
}

func TestDeriveDRBAccountsReturnsLeaderAndThreeRegulars(t *testing.T) {
	leader, regulars, err := DeriveDRBAccounts(drbTestMnemonic)
	if err != nil {
		t.Fatalf("DeriveDRBAccounts error: %v", err)
	}
	if leader == "" {
		t.Error("leader key is empty")
	}
	if len(regulars) != 3 {
		t.Errorf("expected 3 regulars, got %d", len(regulars))
	}
	for i, r := range regulars {
		if r == "" {
			t.Errorf("regular[%d] key is empty", i)
		}
	}
}

func TestDeriveDRBAccountsLeaderMatchesAdmin(t *testing.T) {
	// DRB leader (BIP44 index 0) must match the admin account (also index 0).
	admin, _, _, _, _, err := DeriveRoleAccounts(drbTestMnemonic)
	if err != nil {
		t.Fatalf("DeriveRoleAccounts error: %v", err)
	}
	leader, _, err := DeriveDRBAccounts(drbTestMnemonic)
	if err != nil {
		t.Fatalf("DeriveDRBAccounts error: %v", err)
	}
	if admin != leader {
		t.Errorf("DRB leader key (%s...) does not match admin key (%s...)", leader[:8], admin[:8])
	}
}

func TestDeriveDRBAccountsRegularsAreDistinct(t *testing.T) {
	_, regulars, err := DeriveDRBAccounts(drbTestMnemonic)
	if err != nil {
		t.Fatalf("DeriveDRBAccounts error: %v", err)
	}
	seen := map[string]bool{}
	for i, r := range regulars {
		if seen[r] {
			t.Errorf("regular[%d] key is duplicate", i)
		}
		seen[r] = true
	}
}

func TestDeriveDRBAccountsInvalidMnemonic(t *testing.T) {
	_, _, err := DeriveDRBAccounts("not a valid mnemonic")
	if err == nil {
		t.Error("expected error for invalid mnemonic")
	}
}
