package query

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

func init() {
	Register(pipelinesQueryDef)
}

var pipelinesQueryDef = &QueryDef{
	Name:        "pipelines",
	Description: "Query recent pipeline runs",
	Params: []QueryParam{
		{Name: "project", Type: "string", Required: false},
		{Name: "status", Type: "string", Required: false},
		{Name: "limit", Type: "int", Required: false, Default: "20"},
	},
	Execute: executePipelinesQuery,
}

// PipelineResult represents a single pipeline query result.
type PipelineResult struct {
	ID            int    `json:"id"`
	Status        string `json:"status"`
	BlueprintID   int    `json:"blueprintId,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	BeganAt       string `json:"beganAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	FinishedTasks int    `json:"finishedTasks"`
	TotalTasks    int    `json:"totalTasks"`
	Message       string `json:"message,omitempty"`
}

func executePipelinesQuery(client *devlake.Client, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	var blueprintID int
	if projectName, ok := params["project"].(string); ok && projectName != "" {
		proj, err := client.GetProject(projectName)
		if err != nil {
			return nil, fmt.Errorf("getting project %q: %w", projectName, err)
		}
		if proj.Blueprint != nil {
			blueprintID = proj.Blueprint.ID
		} else {
			return nil, fmt.Errorf("project %q has no blueprint", projectName)
		}
	}

	status := ""
	if s, ok := params["status"].(string); ok {
		status = s
	}

	limit := 20
	if l, ok := params["limit"].(int); ok {
		limit = l
	} else if l, ok := params["limit"].(string); ok {
		var parsedLimit int
		n, err := fmt.Sscanf(l, "%d", &parsedLimit)
		if err != nil || n != 1 {
			return nil, fmt.Errorf("invalid limit value %q: must be a valid integer", l)
		}
		limit = parsedLimit
	}

	// Query pipelines via API
	resp, err := client.ListPipelines(status, blueprintID, 1, limit)
	if err != nil {
		return nil, fmt.Errorf("listing pipelines: %w", err)
	}

	// Transform to output format
	results := make([]PipelineResult, len(resp.Pipelines))
	for i, p := range resp.Pipelines {
		results[i] = PipelineResult{
			ID:            p.ID,
			Status:        p.Status,
			BlueprintID:   p.BlueprintID,
			CreatedAt:     p.CreatedAt,
			BeganAt:       p.BeganAt,
			FinishedAt:    p.FinishedAt,
			FinishedTasks: p.FinishedTasks,
			TotalTasks:    p.TotalTasks,
			Message:       p.Message,
		}
	}

	return results, nil
}
