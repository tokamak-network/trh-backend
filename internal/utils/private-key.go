package utils

import "strings"

// TrimPrivateKey removes the "0x" prefix from a private key string if present
func TrimPrivateKey(privateKey string) string {
	return strings.TrimPrefix(privateKey, "0x")
}
