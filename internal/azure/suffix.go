package azure

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Suffix generates a 5-character lowercase suffix from a resource group name
// using SHA-256 hashing. This provides deterministic, unique suffixes.
func Suffix(resourceGroupName string) string {
	hash := sha256.Sum256([]byte(resourceGroupName))
	hex := fmt.Sprintf("%x", hash)
	return strings.ToLower(hex[:5])
}
