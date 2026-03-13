package query

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

func init() {
	Register(doraQueryDef)
}

var doraQueryDef = &QueryDef{
	Name:        "dora",
	Description: "Query DORA metrics (limited by available API data)",
	Params: []QueryParam{
		{Name: "project", Type: "string", Required: true},
		{Name: "timeframe", Type: "string", Required: false, Default: "30d"},
	},
	Execute: executeDoraQuery,
}

// DoraResult represents DORA metrics that can be retrieved from available APIs.
// NOTE: Full DORA calculations require direct database access (which DevLake doesn't
// expose to external clients). This returns what's available via REST API.
type DoraResult struct {
	Project       string                 `json:"project"`
	Timeframe     string                 `json:"timeframe"`
	AvailableData map[string]interface{} `json:"availableData"`
	Limitations   string                 `json:"limitations"`
}

func executeDoraQuery(client *devlake.Client, params map[string]interface{}) (interface{}, error) {
	projectName, ok := params["project"].(string)
	if !ok || projectName == "" {
		return nil, fmt.Errorf("project parameter is required")
	}

	timeframe := "30d"
	if tf, ok := params["timeframe"].(string); ok && tf != "" {
		timeframe = tf
	}

	// Get project info (this is what's available via API)
	proj, err := client.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("getting project %q: %w", projectName, err)
	}

	availableData := map[string]interface{}{
		"projectName":        proj.Name,
		"projectDescription": proj.Description,
		"enabledMetrics":     proj.Metrics,
	}

	if proj.Blueprint != nil {
		availableData["blueprintId"] = proj.Blueprint.ID
		availableData["blueprintName"] = proj.Blueprint.Name
		availableData["syncSchedule"] = proj.Blueprint.CronConfig
	}

	result := DoraResult{
		Project:       projectName,
		Timeframe:     timeframe,
		AvailableData: availableData,
		Limitations: "DORA metric calculations (deployment frequency, lead time, change failure rate, MTTR) " +
			"require SQL queries against DevLake's domain layer tables. DevLake does not expose database " +
			"credentials or a metrics API endpoint. These calculations are currently only available in " +
			"Grafana dashboards. To compute DORA metrics via CLI, DevLake would need to add a /metrics " +
			"or /dora API endpoint that returns pre-calculated values.",
	}

	return result, nil
}
