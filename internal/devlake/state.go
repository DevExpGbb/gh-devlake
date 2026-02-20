package devlake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State represents the persisted deployment/connection state.
type State struct {
	DeployedAt              string            `json:"deployedAt"`
	Method                  string            `json:"method"`
	Endpoints               StateEndpoints    `json:"endpoints"`
	Connections             []StateConnection `json:"connections,omitempty"`
	ConnectionsConfiguredAt string            `json:"connectionsConfiguredAt,omitempty"`
	Project                 *StateProject     `json:"project,omitempty"`
	ScopesConfiguredAt      string            `json:"scopesConfiguredAt,omitempty"`
}

// StateProject records project and blueprint info after scope configuration.
type StateProject struct {
	Name         string   `json:"name"`
	BlueprintID  int      `json:"blueprintId"`
	Repos        []string `json:"repos,omitempty"`
	Organization string   `json:"organization,omitempty"`
}

// StateEndpoints contains service URLs.
type StateEndpoints struct {
	Backend  string `json:"backend"`
	Grafana  string `json:"grafana,omitempty"`
	ConfigUI string `json:"configUi,omitempty"`
}

// StateConnection records a created connection.
type StateConnection struct {
	Plugin       string `json:"plugin"`
	ConnectionID int    `json:"connectionId"`
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	Enterprise   string `json:"enterprise,omitempty"`
}

// LoadState reads a state file from disk. Returns nil if not found.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// FindStateFile finds the first existing state file in the working directory.
// Returns the path and loaded state, or creates a new local state.
func FindStateFile(apiURL, grafanaURL string) (string, *State) {
	cwd, _ := os.Getwd()

	for _, name := range []string{".devlake-azure.json", ".devlake-local.json"} {
		path := filepath.Join(cwd, name)
		state, err := LoadState(path)
		if err == nil && state != nil {
			return path, state
		}
	}

	// Create new local state
	path := filepath.Join(cwd, ".devlake-local.json")
	state := &State{
		DeployedAt: time.Now().Format(time.RFC3339),
		Method:     "local",
		Endpoints: StateEndpoints{
			Backend: apiURL,
			Grafana: grafanaURL,
		},
	}
	return path, state
}

// SaveState writes state to disk, merging with any existing fields in the file
// (e.g. Azure deployment metadata) that the State struct doesn't model.
func SaveState(path string, state *State) error {
	// Load existing raw JSON to preserve fields not in the State struct
	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Marshal State into a map
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	var stateMap map[string]any
	if err := json.Unmarshal(stateBytes, &stateMap); err != nil {
		return err
	}

	// Overlay State fields onto existing (preserves resourceGroup, resources, etc.)
	for k, v := range stateMap {
		existing[k] = v
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// UpdateConnections updates the connections in the state and saves to disk.
func UpdateConnections(path string, state *State, connections []StateConnection) error {
	state.Connections = connections
	state.ConnectionsConfiguredAt = time.Now().Format(time.RFC3339)
	return SaveState(path, state)
}

// PrintState prints a human-readable summary of the state.
func PrintState(state *State) {
	fmt.Printf("  Backend:   %s\n", state.Endpoints.Backend)
	if state.Endpoints.Grafana != "" {
		fmt.Printf("  Grafana:   %s\n", state.Endpoints.Grafana)
	}
	if len(state.Connections) > 0 {
		fmt.Println("  Connections:")
		for _, c := range state.Connections {
			fmt.Printf("    %s: ID=%d Name=%q\n", c.Plugin, c.ConnectionID, c.Name)
		}
	}
}
