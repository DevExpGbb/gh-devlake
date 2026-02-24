package devlake

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiscoveryResult contains the discovered DevLake instance details.
type DiscoveryResult struct {
	URL         string
	GrafanaURL  string
	ConfigUIURL string
	Source      string // "parameter", "statefile", "localhost"
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
		grafanaURL, configUIURL := inferLocalCompanionURLs(url)
		return &DiscoveryResult{URL: url, GrafanaURL: grafanaURL, ConfigUIURL: configUIURL, Source: "parameter"}, nil
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
		url      string
		grafana  string
		configUI string
	}{
		{"http://localhost:8080", "http://localhost:3002", "http://localhost:4000"},
		{"http://localhost:8085", "http://localhost:3004", "http://localhost:4004"},
	}
	for _, c := range candidates {
		if err := pingURL(c.url); err == nil {
			return &DiscoveryResult{
				URL:         c.url,
				GrafanaURL:  c.grafana,
				ConfigUIURL: c.configUI,
				Source:      "localhost",
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
	state, err := LoadState(path)
	if err != nil || state == nil {
		return nil
	}

	url := strings.TrimRight(state.Endpoints.Backend, "/")
	if url == "" {
		return nil
	}

	if err := pingURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "   ⚠️  Found DevLake URL in %s: %s\n", filepath.Base(path), url)
		fmt.Fprintf(os.Stderr, "      Could not reach /ping: %v\n", err)
		return nil
	}

	return &DiscoveryResult{
		URL:         url,
		GrafanaURL:  state.Endpoints.Grafana,
		ConfigUIURL: state.Endpoints.ConfigUI,
		Source:      "statefile",
	}
}

func inferLocalCompanionURLs(backendURL string) (grafanaURL, configUIURL string) {
	// When the backend is running on a well-known localhost port, the sibling
	// services are typically on matching well-known ports.
	//
	// This is intentionally conservative: only infer for localhost.
	if strings.HasPrefix(backendURL, "http://localhost:8080") {
		return "http://localhost:3002", "http://localhost:4000"
	}
	if strings.HasPrefix(backendURL, "http://localhost:8085") {
		return "http://localhost:3004", "http://localhost:4004"
	}
	return "", ""
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
