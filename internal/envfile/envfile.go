// Package envfile loads key-value pairs from .devlake.env files.
//
// The file format is simple: one KEY=VALUE per line, # comments, blank lines ignored.
// Values may be optionally quoted with single or double quotes.
// This package does NOT set process environment variables — it returns a map
// so that secrets remain scoped and are not leaked into child processes.
package envfile

import (
	"bufio"
	"os"
	"strings"
)

// Load reads a .devlake.env file and returns a map of key-value pairs.
// Returns an empty map (not an error) if the file does not exist.
func Load(path string) (map[string]string, error) {
	result := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '='
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Strip surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		if value != "" {
			result[key] = value
		}
	}

	return result, scanner.Err()
}

// Delete removes the env file from disk. Returns nil if the file does not exist.
func Delete(path string) error {
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
