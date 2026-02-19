// Package devlake provides an HTTP client for the DevLake REST API.
package devlake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps HTTP calls to the DevLake backend API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a Client for the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping checks if the DevLake backend is reachable.
func (c *Client) Ping() error {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/ping")
	if err != nil {
		return fmt.Errorf("cannot reach DevLake at %s/ping: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DevLake returned status %d from /ping", resp.StatusCode)
	}
	return nil
}

// Connection represents a DevLake plugin connection.
type Connection struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	Enterprise   string `json:"enterprise,omitempty"`
}

// ConnectionCreateRequest is the payload for creating a GitHub or Copilot connection.
type ConnectionCreateRequest struct {
	Name                  string `json:"name"`
	Endpoint              string `json:"endpoint"`
	Proxy                 string `json:"proxy,omitempty"`
	AuthMethod            string `json:"authMethod"`
	Token                 string `json:"token"`
	EnableGraphql         bool   `json:"enableGraphql,omitempty"`
	RateLimitPerHour      int    `json:"rateLimitPerHour"`
	Organization          string `json:"organization,omitempty"`
	Enterprise            string `json:"enterprise,omitempty"`
	TokenExpiresAt        string `json:"tokenExpiresAt,omitempty"`
	RefreshTokenExpiresAt string `json:"refreshTokenExpiresAt,omitempty"`
}

// ConnectionTestRequest is the payload for testing a connection before creating.
type ConnectionTestRequest struct {
	Endpoint         string `json:"endpoint"`
	AuthMethod       string `json:"authMethod"`
	Token            string `json:"token"`
	EnableGraphql    bool   `json:"enableGraphql,omitempty"`
	RateLimitPerHour int    `json:"rateLimitPerHour"`
	Proxy            string `json:"proxy"`
	Organization     string `json:"organization,omitempty"`
	Enterprise       string `json:"enterprise,omitempty"`
}

// ConnectionTestResult is the response from testing a connection.
type ConnectionTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ListConnections returns all connections for a plugin (e.g. "github", "gh-copilot").
func (c *Client) ListConnections(plugin string) ([]Connection, error) {
	resp, err := c.HTTPClient.Get(fmt.Sprintf("%s/plugins/%s/connections", c.BaseURL, plugin))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list connections returned %d: %s", resp.StatusCode, body)
	}

	var conns []Connection
	if err := json.Unmarshal(body, &conns); err != nil {
		return nil, err
	}
	return conns, nil
}

// FindConnectionByName returns the first connection matching the given name, or nil.
func (c *Client) FindConnectionByName(plugin, name string) (*Connection, error) {
	conns, err := c.ListConnections(plugin)
	if err != nil {
		return nil, err
	}
	for _, conn := range conns {
		if conn.Name == name {
			return &conn, nil
		}
	}
	return nil, nil
}

// TestConnection tests connection parameters before creating.
func (c *Client) TestConnection(plugin string, req *ConnectionTestRequest) (*ConnectionTestResult, error) {
	return doPost[ConnectionTestResult](c, fmt.Sprintf("/plugins/%s/test", plugin), req)
}

// CreateConnection creates a new connection for the given plugin.
func (c *Client) CreateConnection(plugin string, req *ConnectionCreateRequest) (*Connection, error) {
	return doPost[Connection](c, fmt.Sprintf("/plugins/%s/connections", plugin), req)
}

// DeleteConnection deletes a plugin connection by ID.
func (c *Client) DeleteConnection(plugin string, connID int) error {
	url := fmt.Sprintf("%s/plugins/%s/connections/%d", c.BaseURL, plugin, connID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete connection returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

// TestSavedConnection tests an already-created connection by ID.
func (c *Client) TestSavedConnection(plugin string, connID int) (*ConnectionTestResult, error) {
	url := fmt.Sprintf("%s/plugins/%s/connections/%d/test", c.BaseURL, plugin, connID)

	reqBody := bytes.NewBufferString("{}")
	resp, err := c.HTTPClient.Post(url, "application/json", reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result ConnectionTestResult
	if err := json.Unmarshal(body, &result); err != nil {
		// Non-JSON response is ok — treat as success if status 200
		if resp.StatusCode == http.StatusOK {
			return &ConnectionTestResult{Success: true}, nil
		}
		return nil, fmt.Errorf("test connection returned %d: %s", resp.StatusCode, body)
	}
	return &result, nil
}

// HealthStatus represents the response from /health or /ping.
type HealthStatus struct {
	Status string `json:"status"`
}

// Health returns the DevLake health status.
func (c *Client) Health() (*HealthStatus, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/ping")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var hs HealthStatus
	_ = json.Unmarshal(body, &hs)
	if resp.StatusCode == http.StatusOK {
		if hs.Status == "" {
			hs.Status = "ok"
		}
		return &hs, nil
	}
	return nil, fmt.Errorf("health check returned %d: %s", resp.StatusCode, body)
}

// doPost is a generic helper for POST requests that return JSON.
func doPost[T any](c *Client, path string, payload any) (*T, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := c.BaseURL + path
	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("POST %s returned %d: %s", path, resp.StatusCode, body)
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// doGet is a generic helper for GET requests that return JSON.
func doGet[T any](c *Client, path string) (*T, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d: %s", path, resp.StatusCode, body)
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// doPut is a generic helper for PUT requests that return JSON.
func doPut[T any](c *Client, path string, payload any) (*T, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, c.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("PUT %s returned %d: %s", path, resp.StatusCode, body)
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// doPatch is a generic helper for PATCH requests that return JSON.
func doPatch[T any](c *Client, path string, payload any) (*T, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPatch, c.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PATCH %s returned %d: %s", path, resp.StatusCode, body)
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateScopeConfig creates a scope config for a plugin connection.
func (c *Client) CreateScopeConfig(plugin string, connID int, cfg *ScopeConfig) (*ScopeConfig, error) {
	return doPost[ScopeConfig](c, fmt.Sprintf("/plugins/%s/connections/%d/scope-configs", plugin, connID), cfg)
}

// ListScopeConfigs returns all scope configs for a plugin connection.
func (c *Client) ListScopeConfigs(plugin string, connID int) ([]ScopeConfig, error) {
	result, err := doGet[[]ScopeConfig](c, fmt.Sprintf("/plugins/%s/connections/%d/scope-configs", plugin, connID))
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// PutScopes batch-upserts scopes for a plugin connection.
func (c *Client) PutScopes(plugin string, connID int, req *ScopeBatchRequest) error {
	_, err := doPut[json.RawMessage](c, fmt.Sprintf("/plugins/%s/connections/%d/scopes", plugin, connID), req)
	return err
}

// CreateProject creates a new DevLake project.
func (c *Client) CreateProject(project *Project) (*Project, error) {
	return doPost[Project](c, "/projects", project)
}

// GetProject retrieves a project by name.
func (c *Client) GetProject(name string) (*Project, error) {
	return doGet[Project](c, fmt.Sprintf("/projects/%s", name))
}

// PatchBlueprint updates a blueprint by ID.
func (c *Client) PatchBlueprint(id int, patch *BlueprintPatch) (*Blueprint, error) {
	return doPatch[Blueprint](c, fmt.Sprintf("/blueprints/%d", id), patch)
}

// TriggerBlueprint triggers a blueprint to run and returns the pipeline.
func (c *Client) TriggerBlueprint(id int) (*Pipeline, error) {
	return doPost[Pipeline](c, fmt.Sprintf("/blueprints/%d/trigger", id), struct{}{})
}

// GetPipeline retrieves a pipeline by ID.
func (c *Client) GetPipeline(id int) (*Pipeline, error) {
	return doGet[Pipeline](c, fmt.Sprintf("/pipelines/%d", id))
}

// TriggerMigration triggers the DevLake database migration endpoint.
func (c *Client) TriggerMigration() error {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/proceed-db-migration")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
