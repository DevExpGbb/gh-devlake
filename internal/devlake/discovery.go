package devlake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// stateFile represents the structure of .devlake-azure.json or .devlake-local.json.
type stateFile struct {
	Endpoints struct {
		Backend string `json:"backend"`
		Grafana string `json:"grafana"`
	} `json:"endpoints"`
	Connections []struct {
		Plugin       string `json:"plugin"`
		ConnectionID int    `json:"connectionId"`
		Name         string `json:"name"`
	} `json:"connections"`
}

// DiscoveryResult contains the discovered DevLake instance details.
type DiscoveryResult struct {
	URL        string
	GrafanaURL string
	Source     string // "parameter", "statefile", "localhost"
}

// Discover finds a running DevLake instance by checking multiple sources.
// Priority: explicit URL → state files → well-known localhost ports.
func Discover(explicitURL string) (*DiscoveryResult, error) {
	// 1. Explicit URL
	if explicitURL != "" {
		url := strings.TrimRight(explicitURL, "/")
		if err := pingURL(url); err != nil {
			return nil, fmt.Errorf("cannot reach DevLake at %s: %w", url, err)
		}
		return &DiscoveryResult{URL: url, Source: "parameter"}, nil
	}

	// 2. State files
	cwd, _ := os.Getwd()
	for _, name := range []string{".devlake-azure.json", ".devlake-local.json"} {
		path := filepath.Join(cwd, name)
		if result := tryStateFile(path); result != nil {
			return result, nil
		}
	}

	// 3. Well-known local ports
	candidates := []struct {
		url     string
		grafana string
	}{
		{"http://localhost:8080", "http://localhost:3002"},
		{"http://localhost:8085", "http://localhost:3004"},
	}
	for _, c := range candidates {
		if err := pingURL(c.url); err == nil {
			return &DiscoveryResult{
				URL:        c.url,
				GrafanaURL: c.grafana,
				Source:     "localhost",
			}, nil
		}
	}

	return nil, fmt.Errorf("could not find a running DevLake instance.\n" +
		"Checked: state files, localhost:8080, localhost:8085.\n\n" +
		"To deploy a new instance:\n" +
		"  gh devlake deploy local     # Docker Compose on this machine\n" +
		"  gh devlake deploy azure     # Azure Container Apps\n\n" +
		"Or specify an existing instance with --url <DevLake API URL>")
}

func tryStateFile(path string) *DiscoveryResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil
	}

	url := strings.TrimRight(sf.Endpoints.Backend, "/")
	if url == "" {
		return nil
	}

	if err := pingURL(url); err != nil {
		return nil
	}

	return &DiscoveryResult{
		URL:        url,
		GrafanaURL: sf.Endpoints.Grafana,
		Source:     "statefile",
	}
}

func pingURL(baseURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/ping")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
