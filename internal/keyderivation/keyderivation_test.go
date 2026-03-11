package keyderivation_test

import (
	"strings"
	"testing"

	"github.com/tokamak-network/trh-backend/internal/keyderivation"
)

// knownMnemonic is the well-known Hardhat/Anvil test mnemonic.
// The expected addresses below are the canonical Hardhat account addresses (account 0–3)
// verified at https://iancoleman.io/bip39/ (BIP44, coin=60, path m/44'/60'/0'/0)
// and cross-checked against the Hardhat source (packages/hardhat-core/src/internal/core/config/default-config.ts).
const knownMnemonic = "test test test test test test test test test test test junk"

// referenceKeys holds externally verified BIP44 addresses for the Hardhat test mnemonic.
var referenceKeys = []struct {
	index   uint32
	address string // checksummed
}{
	{keyderivation.AdminIndex, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"},
	{keyderivation.SequencerIndex, "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"},
	{keyderivation.BatcherIndex, "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"},
	{keyderivation.ProposerIndex, "0x90F79bf6EB2c4f870365E785982E1f101E93b906"},
}

func TestDeriveOperatorKeys_AddressesMatchReference(t *testing.T) {
	admin, seq, batcher, proposer, err := keyderivation.DeriveOperatorKeys(knownMnemonic, "")
	if err != nil {
		t.Fatalf("DeriveOperatorKeys failed: %v", err)
	}

	got := []keyderivation.DerivedKey{admin, seq, batcher, proposer}
	for i, ref := range referenceKeys {
		if got[i].Address != ref.address {
			t.Errorf("index %d: address = %q, want %q", ref.index, got[i].Address, ref.address)
		}
	}
}

func TestDeriveOperatorKeys_PrivateKeyFormat(t *testing.T) {
	admin, _, _, _, err := keyderivation.DeriveOperatorKeys(knownMnemonic, "")
	if err != nil {
		t.Fatalf("DeriveOperatorKeys failed: %v", err)
	}

	if len(admin.PrivateKey) != 64 {
		t.Errorf("private key length = %d, want 64", len(admin.PrivateKey))
	}
	if strings.HasPrefix(admin.PrivateKey, "0x") {
		t.Errorf("private key should not have 0x prefix, got: %s", admin.PrivateKey)
	}
}

func TestDeriveOperatorKeys_WithPassphrase(t *testing.T) {
	admin1, _, _, _, err1 := keyderivation.DeriveOperatorKeys(knownMnemonic, "")
	admin2, _, _, _, err2 := keyderivation.DeriveOperatorKeys(knownMnemonic, "extra-passphrase")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected error: %v %v", err1, err2)
	}

	if admin1.Address == admin2.Address {
		t.Error("passphrase should produce different addresses")
	}
}

func TestDeriveOperatorKeys_InvalidMnemonic(t *testing.T) {
	_, _, _, _, err := keyderivation.DeriveOperatorKeys("invalid mnemonic phrase that is not bip39 valid", "")
	if err == nil {
		t.Error("expected error for invalid mnemonic, got nil")
	}
}

func TestDeriveKeyAtIndex_MatchesOperatorKeys(t *testing.T) {
	admin, seq, batcher, proposer, err := keyderivation.DeriveOperatorKeys(knownMnemonic, "")
	if err != nil {
		t.Fatalf("DeriveOperatorKeys failed: %v", err)
	}

	expected := map[uint32]keyderivation.DerivedKey{
		keyderivation.AdminIndex:     admin,
		keyderivation.SequencerIndex: seq,
		keyderivation.BatcherIndex:   batcher,
		keyderivation.ProposerIndex:  proposer,
	}

	for idx, want := range expected {
		got, err := keyderivation.DeriveKeyAtIndex(knownMnemonic, "", idx)
		if err != nil {
			t.Fatalf("DeriveKeyAtIndex(%d) failed: %v", idx, err)
		}
		if got.Address != want.Address {
			t.Errorf("index %d: address = %q, want %q", idx, got.Address, want.Address)
		}
		if got.PrivateKey != want.PrivateKey {
			t.Errorf("index %d: private key mismatch", idx)
		}
	}
}

func TestDeriveKeyAtIndex_InvalidMnemonic(t *testing.T) {
	_, err := keyderivation.DeriveKeyAtIndex("not valid", "", 0)
	if err == nil {
		t.Error("expected error for invalid mnemonic")
	}
}
