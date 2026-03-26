package thanos

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

// DeriveRoleAccounts derives 5 role accounts from a BIP39 mnemonic.
// Derivation path: m/44'/60'/0'/0/{0,1,2,3,4}
// Returns hex-encoded private keys (no 0x prefix) for admin, sequencer, batcher, proposer, challenger.
func DeriveRoleAccounts(mnemonic string) (admin, sequencer, batcher, proposer, challenger string, err error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return "", "", "", "", "", fmt.Errorf("invalid BIP39 mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")

	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("failed to derive master key: %w", err)
	}

	// Derive m/44'/60'/0'/0 — BIP44 path for Ethereum external addresses
	path := []uint32{
		bip32.FirstHardenedChild + 44, // purpose
		bip32.FirstHardenedChild + 60, // coin type: Ethereum
		bip32.FirstHardenedChild + 0,  // account
		0,                             // change (external)
	}

	key := masterKey
	for _, idx := range path {
		key, err = key.NewChildKey(idx)
		if err != nil {
			return "", "", "", "", "", fmt.Errorf("failed to derive child key at path index %d: %w", idx, err)
		}
	}

	keys := make([]string, 5)
	for i := 0; i < 5; i++ {
		child, deriveErr := key.NewChildKey(uint32(i))
		if deriveErr != nil {
			return "", "", "", "", "", fmt.Errorf("failed to derive key at address index %d: %w", i, deriveErr)
		}

		privKey, ecdsaErr := crypto.ToECDSA(child.Key)
		if ecdsaErr != nil {
			return "", "", "", "", "", fmt.Errorf("failed to convert key to ECDSA at index %d: %w", i, ecdsaErr)
		}
		keys[i] = hex.EncodeToString(crypto.FromECDSA(privKey))
	}

	return keys[0], keys[1], keys[2], keys[3], keys[4], nil
}
