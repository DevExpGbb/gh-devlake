package query

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

func init() {
	Register(copilotQueryDef)
}

var copilotQueryDef = &QueryDef{
	Name:        "copilot",
	Description: "Query GitHub Copilot metrics (limited by available API data)",
	Params: []QueryParam{
		{Name: "project", Type: "string", Required: true},
		{Name: "timeframe", Type: "string", Required: false, Default: "30d"},
	},
	Execute: executeCopilotQuery,
}

// CopilotResult represents Copilot metrics that can be retrieved from available APIs.
// NOTE: Copilot usage metrics (acceptance rates, language breakdowns) are stored in
// _tool_gh_copilot_* tables but not exposed via REST API.
type CopilotResult struct {
	Project       string                 `json:"project"`
	Timeframe     string                 `json:"timeframe"`
	AvailableData map[string]interface{} `json:"availableData"`
	Limitations   string                 `json:"limitations"`
}

func executeCopilotQuery(client *devlake.Client, params map[string]interface{}) (interface{}, error) {
	projectName, ok := params["project"].(string)
	if !ok || projectName == "" {
		return nil, fmt.Errorf("project parameter is required")
	}

	timeframe := "30d"
	if tf, ok := params["timeframe"].(string); ok && tf != "" {
		timeframe = tf
	}

	// Get project info
	proj, err := client.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("getting project %q: %w", projectName, err)
	}

	// Check if gh-copilot plugin is configured
	connections, err := client.ListConnections("gh-copilot")
	if err != nil {
		return nil, fmt.Errorf("listing gh-copilot connections: %w", err)
	}

	availableData := map[string]interface{}{
		"projectName":             proj.Name,
		"copilotConnectionsFound": len(connections),
	}

	if len(connections) > 0 {
		availableData["connections"] = connections
	}

	result := CopilotResult{
		Project:       projectName,
		Timeframe:     timeframe,
		AvailableData: availableData,
		Limitations: "GitHub Copilot usage metrics (total seats, active users, acceptance rates, language " +
			"breakdowns, editor usage) are stored in _tool_gh_copilot_* tables and visualized in Grafana " +
			"dashboards, but DevLake does not expose a /metrics or /copilot API endpoint. To retrieve " +
			"Copilot metrics via CLI, DevLake would need to add a metrics API that returns aggregated " +
			"Copilot usage data.",
	}

	return result, nil
}
