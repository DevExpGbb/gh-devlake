// Package query provides an extensible abstraction for querying DevLake data.
// Instead of direct SQL queries (DevLake doesn't expose DB credentials), this
// package defines queries as API endpoint patterns with client-side transformations.
package query

import (
	"github.com/DevExpGBB/gh-devlake/internal/devlake"
)

// QueryDef describes a reusable, parameterized query against DevLake's API.
// Unlike the original SQL-based design, this uses HTTP API endpoints since
// DevLake doesn't expose database credentials to external clients.
type QueryDef struct {
	Name        string            // e.g. "pipelines", "dora_metrics"
	Description string            // human-readable description
	Params      []QueryParam      // declared parameters with types and defaults
	Execute     QueryExecuteFunc  // function that executes the query
}

// QueryParam describes a parameter for a query.
type QueryParam struct {
	Name     string // parameter name
	Type     string // "string", "int", "duration"
	Required bool   // whether the parameter is required
	Default  string // default value if not provided
}

// QueryExecuteFunc is the signature for query execution functions.
// It takes a client, parameters, and returns results or an error.
type QueryExecuteFunc func(client *devlake.Client, params map[string]interface{}) (interface{}, error)

// QueryResult wraps the output of a query execution.
type QueryResult struct {
	Data   interface{}       // the actual result data
	Metadata map[string]string // optional metadata about the query
}
