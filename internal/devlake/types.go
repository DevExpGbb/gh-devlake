package devlake

import (
	"encoding/json"
	"strconv"
)

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

// GitLabProjectScope represents a GitLab project scope entry for PUT /scopes.
type GitLabProjectScope struct {
	GitlabID          int    `json:"gitlabId"`
	ConnectionID      int    `json:"connectionId"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"pathWithNamespace"`
	HTTPURLToRepo     string `json:"httpUrlToRepo,omitempty"`
	SSHURLToRepo      string `json:"sshUrlToRepo,omitempty"`
	ScopeConfigID     int    `json:"scopeConfigId,omitempty"`
}

// JenkinsJobScope represents a Jenkins job scope entry.
type JenkinsJobScope struct {
	ConnectionID int    `json:"connectionId"`
	FullName     string `json:"fullName"`
	Name         string `json:"name"`
}

// JiraBoardScope represents a Jira board scope entry for PUT /scopes.
type JiraBoardScope struct {
	BoardID      uint64 `json:"boardId"`
	ConnectionID int    `json:"connectionId"`
	Name         string `json:"name"`
}

// BitbucketRepoScope represents a Bitbucket Cloud repository scope entry for PUT /scopes.
// BitbucketID holds the repository full name (workspace/repo-slug), which is the
// canonical scope identifier used by the DevLake Bitbucket plugin.
type BitbucketRepoScope struct {
	BitbucketID  string `json:"bitbucketId"`
	ConnectionID int    `json:"connectionId"`
	Name         string `json:"name"`
	FullName     string `json:"fullName"`
	CloneURL     string `json:"cloneUrl,omitempty"`
	HTMLURL      string `json:"htmlUrl,omitempty"`
}

// SonarQubeProjectScope represents a SonarQube project scope entry for PUT /scopes.
type SonarQubeProjectScope struct {
	ConnectionID int    `json:"connectionId"`
	ProjectKey   string `json:"projectKey"`
	Name         string `json:"name"`
}

// ArgoCDAppScope represents an ArgoCD application scope entry for PUT /scopes.
type ArgoCDAppScope struct {
	ConnectionID int    `json:"connectionId"`
	Name         string `json:"name"`
}

// ScopeBatchRequest is the payload for PUT /scopes (batch upsert).
type ScopeBatchRequest struct {
	Data []any `json:"data"`
}

// ScopeListWrapper wraps a scope object as returned by the DevLake GET scopes API.
// The API nests each scope inside a "scope" key: { "scope": { ... } }.
// RawScope preserves the full plugin-specific payload for generic ID extraction.
type ScopeListWrapper struct {
	RawScope json.RawMessage            `json:"scope"`
	parsed   map[string]json.RawMessage // lazily populated by parseScope
}

// parseScope unmarshals RawScope into a map exactly once per wrapper instance,
// caching the result so callers that invoke both ScopeName and ScopeFullName on
// the same item do not unmarshal the same JSON twice.
func (w *ScopeListWrapper) parseScope() map[string]json.RawMessage {
	if w.parsed == nil {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(w.RawScope, &m); err != nil || m == nil {
			m = make(map[string]json.RawMessage)
		}
		w.parsed = m
	}
	return w.parsed
}

// ScopeName returns the display name from the raw scope JSON (checks "fullName" then "name").
// Empty string values are skipped so the next candidate key is tried.
// Parsing is cached via parseScope() so calling ScopeName and ScopeFullName on the
// same instance only unmarshals the JSON once.
func (w *ScopeListWrapper) ScopeName() string {
	m := w.parseScope()
	for _, key := range []string{"fullName", "name"} {
		if v, ok := m[key]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// ScopeFullName returns the "fullName" field from the raw scope JSON, or "".
// An empty string value is treated as absent (returns "").
func (w *ScopeListWrapper) ScopeFullName() string {
	m := w.parseScope()
	if v, ok := m["fullName"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}

// ExtractScopeID extracts the scope ID from a raw JSON scope object using the
// given field name.  It tries to decode the value as a string first, then as
// an integer (converted to its decimal string representation).
func ExtractScopeID(raw json.RawMessage, fieldName string) string {
	if fieldName == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	v, ok := m[fieldName]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil && s != "" {
		return s
	}
	var n int64
	if err := json.Unmarshal(v, &n); err == nil && n != 0 {
		return strconv.FormatInt(n, 10)
	}
	return ""
}

// ScopeListResponse is the response from GET /plugins/{plugin}/connections/{id}/scopes.
type ScopeListResponse struct {
	Scopes []ScopeListWrapper `json:"scopes"`
	Count  int                `json:"count"`
}

// RemoteScopeChild represents one item (group or scope) from the remote-scope API.
type RemoteScopeChild struct {
	Type     string          `json:"type"` // "group" or "scope"
	ID       string          `json:"id"`
	ParentID string          `json:"parentId"`
	Name     string          `json:"name"`
	FullName string          `json:"fullName"`
	Data     json.RawMessage `json:"data"`
}

// RemoteScopeResponse is the response from GET /plugins/{plugin}/connections/{id}/remote-scopes.
type RemoteScopeResponse struct {
	Children      []RemoteScopeChild `json:"children"`
	NextPageToken string             `json:"nextPageToken"`
}

// Project represents a DevLake project.
type Project struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Metrics     []ProjectMetric `json:"metrics,omitempty"`
	Blueprint   *Blueprint      `json:"blueprint,omitempty"`
}

// ProjectListResponse is the response from GET /projects.
type ProjectListResponse struct {
	Count    int       `json:"count"`
	Projects []Project `json:"projects"`
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
	Mode        string                `json:"mode,omitempty"`
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
