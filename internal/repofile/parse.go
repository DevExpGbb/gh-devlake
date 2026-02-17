// Package repofile parses CSV/TXT files containing GitHub repo names.
package repofile

import (
	"bufio"
	"os"
	"strings"
)

// Parse reads a file with one "owner/repo" per line.
// Lines starting with # are comments. A header row starting with "repo" is skipped.
func Parse(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var repos []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip CSV header row
		if strings.EqualFold(line, "repo") || strings.HasPrefix(strings.ToLower(line), "repo,") {
			continue
		}
		// Take only the first comma-separated field
		if idx := strings.Index(line, ","); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos, scanner.Err()
}
