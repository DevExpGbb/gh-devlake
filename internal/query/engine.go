package query

import (
	"fmt"

	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

// Engine executes queries against DevLake's REST API.
type Engine struct {
	client *devlake.Client
}

// NewEngine creates a new query engine with the given DevLake client.
func NewEngine(client *devlake.Client) *Engine {
	return &Engine{
		client: client,
	}
}

// Execute runs a query with the given parameters.
func (e *Engine) Execute(queryDef *QueryDef, params map[string]interface{}) (interface{}, error) {
	if queryDef == nil {
		return nil, fmt.Errorf("query definition is nil")
	}
	if queryDef.Execute == nil {
		return nil, fmt.Errorf("query %q has no execute function", queryDef.Name)
	}

	// Apply defaults and validate parameters
	for _, param := range queryDef.Params {
		if _, ok := params[param.Name]; !ok {
			// Parameter not provided
			if param.Default != "" {
				// Apply default value
				params[param.Name] = param.Default
			} else if param.Required {
				// Required parameter missing with no default
				return nil, fmt.Errorf("required parameter %q not provided", param.Name)
			}
		}
	}

	// Execute the query
	return queryDef.Execute(e.client, params)
}

// GetClient returns the underlying DevLake client.
func (e *Engine) GetClient() *devlake.Client {
	return e.client
}
