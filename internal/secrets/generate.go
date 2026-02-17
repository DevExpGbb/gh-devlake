// Package secrets generates cryptographic secrets for DevLake configuration.
package secrets

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// EncryptionSecret generates a random uppercase-letter string of the given length.
func EncryptionSecret(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("crypto/rand failed: %w", err)
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}

// MySQLPassword generates a secure random password suitable for MySQL.
// Returns a 20-char string with mixed characters.
func MySQLPassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 16)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("crypto/rand failed: %w", err)
		}
		result[i] = charset[n.Int64()]
	}
	// Append fixed suffix to meet complexity requirements
	return string(result) + "Aa1!", nil
}
