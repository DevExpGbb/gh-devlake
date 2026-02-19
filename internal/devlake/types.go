package devlake

// ScopeConfig represents a DevLake scope configuration (e.g., DORA settings).
type ScopeConfig struct {
	ID                int            `json:"id,omitempty"`
	Name              string         `json:"name"`
	ConnectionID      int            `json:"connectionId"`
	DeploymentPattern string         `json:"deploymentPattern,omitempty"`
	ProductionPattern string         `json:"productionPattern,omitempty"`
	IssueTypeIncident string         `json:"issueTypeIncident,omitempty"`
	Refdiff           *RefdiffConfig `json:"refdiff,omitempty"`
}

// RefdiffConfig holds refdiff tag-matching settings.
type RefdiffConfig struct {
	TagsPattern string `json:"tagsPattern"`
	TagsLimit   int    `json:"tagsLimit"`
	TagsOrder   string `json:"tagsOrder"`
}

// GitHubRepoScope represents a GitHub repository scope entry for PUT /scopes.
type GitHubRepoScope struct {
	GithubID      int    `json:"githubId"`
	ConnectionID  int    `json:"connectionId"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	HTMLURL       string `json:"htmlUrl"`
	CloneURL      string `json:"cloneUrl"`
	ScopeConfigID int    `json:"scopeConfigId,omitempty"`
}

// CopilotScope represents a Copilot organization or enterprise scope entry.
type CopilotScope struct {
	ID           string `json:"id"`
	ConnectionID int    `json:"connectionId"`
	Organization string `json:"organization"`
	Enterprise   string `json:"enterprise,omitempty"`
	Name         string `json:"name"`
	FullName     string `json:"fullName"`
}

// ScopeBatchRequest is the payload for PUT /scopes (batch upsert).
type ScopeBatchRequest struct {
	Data []any `json:"data"`
}

// ScopeListEntry represents a scope entry returned by GET /scopes.
// Fields are the common subset across plugins; plugin-specific fields are ignored.
type ScopeListEntry struct {
	ScopeID   string `json:"scopeId"`
	ScopeName string `json:"scopeName"`
	FullName  string `json:"fullName,omitempty"`
}

// ScopeListResponse is the response from GET /plugins/{plugin}/connections/{id}/scopes.
type ScopeListResponse struct {
	Scopes []ScopeListEntry `json:"scopes"`
	Count  int              `json:"count"`
}

// Project represents a DevLake project.
type Project struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Metrics     []ProjectMetric `json:"metrics,omitempty"`
	Blueprint   *Blueprint      `json:"blueprint,omitempty"`
}

// ProjectMetric enables a metric plugin for a project.
type ProjectMetric struct {
	PluginName string `json:"pluginName"`
	Enable     bool   `json:"enable"`
}

// Blueprint represents a DevLake blueprint (returned from project creation or GET).
type Blueprint struct {
	ID          int                   `json:"id"`
	Name        string                `json:"name,omitempty"`
	Enable      bool                  `json:"enable,omitempty"`
	CronConfig  string                `json:"cronConfig,omitempty"`
	TimeAfter   string                `json:"timeAfter,omitempty"`
	Connections []BlueprintConnection `json:"connections,omitempty"`
}

// BlueprintPatch is the payload for PATCH /blueprints/:id.
type BlueprintPatch struct {
	Enable      *bool                 `json:"enable,omitempty"`
	CronConfig  string                `json:"cronConfig,omitempty"`
	TimeAfter   string                `json:"timeAfter,omitempty"`
	Connections []BlueprintConnection `json:"connections,omitempty"`
}

// BlueprintConnection associates a plugin connection with scopes in a blueprint.
type BlueprintConnection struct {
	PluginName   string           `json:"pluginName"`
	ConnectionID int              `json:"connectionId"`
	Scopes       []BlueprintScope `json:"scopes"`
}

// BlueprintScope identifies a single scope within a blueprint connection.
type BlueprintScope struct {
	ScopeID   string `json:"scopeId"`
	ScopeName string `json:"scopeName"`
}

// Pipeline represents a DevLake pipeline (returned from trigger or GET).
type Pipeline struct {
	ID            int    `json:"id"`
	Status        string `json:"status"`
	FinishedTasks int    `json:"finishedTasks"`
	TotalTasks    int    `json:"totalTasks"`
}
