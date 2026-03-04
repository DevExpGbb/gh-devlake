package devlake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestExtractScopeID tests the generic scope ID extraction helper.
func TestExtractScopeID(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		fieldName string
		want      string
	}{
		{
			name:      "string field",
			raw:       `{"id": "org/repo"}`,
			fieldName: "id",
			want:      "org/repo",
		},
		{
			name:      "integer field",
			raw:       `{"githubId": 12345678}`,
			fieldName: "githubId",
			want:      "12345678",
		},
		{
			name:      "field not present",
			raw:       `{"name": "repo1"}`,
			fieldName: "githubId",
			want:      "",
		},
		{
			name:      "empty field name",
			raw:       `{"id": "x"}`,
			fieldName: "",
			want:      "",
		},
		{
			name:      "zero integer is treated as missing",
			raw:       `{"githubId": 0}`,
			fieldName: "githubId",
			want:      "",
		},
		{
			name:      "empty string is treated as missing",
			raw:       `{"id": ""}`,
			fieldName: "id",
			want:      "",
		},
		{
			name:      "invalid JSON returns empty string",
			raw:       `not json`,
			fieldName: "id",
			want:      "",
		},
		{
			name:      "large integer field",
			raw:       `{"gitlabId": 9876543210}`,
			fieldName: "gitlabId",
			want:      "9876543210",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractScopeID(json.RawMessage(tt.raw), tt.fieldName)
			if got != tt.want {
				t.Errorf("ExtractScopeID(%q, %q) = %q, want %q", tt.raw, tt.fieldName, got, tt.want)
			}
		})
	}
}

// TestScopeListWrapperHelpers tests the ScopeName and ScopeFullName helpers.
func TestScopeListWrapperHelpers(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantName     string
		wantFullName string
	}{
		{
			name:         "fullName takes precedence in ScopeName",
			raw:          `{"name": "repo1", "fullName": "org/repo1"}`,
			wantName:     "org/repo1",
			wantFullName: "org/repo1",
		},
		{
			name:         "name used when fullName absent",
			raw:          `{"name": "repo1"}`,
			wantName:     "repo1",
			wantFullName: "",
		},
		{
			name:         "both empty",
			raw:          `{}`,
			wantName:     "",
			wantFullName: "",
		},
		{
			name:         "invalid JSON",
			raw:          `not json`,
			wantName:     "",
			wantFullName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ScopeListWrapper{RawScope: json.RawMessage(tt.raw)}
			if got := w.ScopeName(); got != tt.wantName {
				t.Errorf("ScopeName() = %q, want %q", got, tt.wantName)
			}
			if got := w.ScopeFullName(); got != tt.wantFullName {
				t.Errorf("ScopeFullName() = %q, want %q", got, tt.wantFullName)
			}
		})
	}
}

// TestListRemoteScopes tests the ListRemoteScopes client method.
func TestListRemoteScopes(t *testing.T) {
	tests := []struct {
		name          string
		groupID       string
		pageToken     string
		statusCode    int
		body          string
		wantErr       bool
		wantChildren  int
		wantNextToken string
		wantPath      string
	}{
		{
			name:          "success without params",
			statusCode:    http.StatusOK,
			body:          `{"children": [{"type": "scope", "id": "123", "name": "repo1"}], "nextPageToken": ""}`,
			wantChildren:  1,
			wantNextToken: "",
			wantPath:      "/plugins/github/connections/1/remote-scopes",
		},
		{
			name:          "success with groupID and pageToken",
			groupID:       "mygroup",
			pageToken:     "tok1",
			statusCode:    http.StatusOK,
			body:          `{"children": [{"type": "group", "id": "g1", "name": "Group 1"}, {"type": "scope", "id": "s1", "name": "Scope 1"}], "nextPageToken": "tok2"}`,
			wantChildren:  2,
			wantNextToken: "tok2",
			wantPath:      "/plugins/github/connections/1/remote-scopes",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": "server error"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if r.URL.Path != tt.wantPath && tt.wantPath != "" {
					t.Errorf("path = %s, want %s", r.URL.Path, tt.wantPath)
				}
				if tt.groupID != "" && r.URL.Query().Get("groupId") != tt.groupID {
					t.Errorf("groupId = %q, want %q", r.URL.Query().Get("groupId"), tt.groupID)
				}
				if tt.pageToken != "" && r.URL.Query().Get("pageToken") != tt.pageToken {
					t.Errorf("pageToken = %q, want %q", r.URL.Query().Get("pageToken"), tt.pageToken)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			result, err := client.ListRemoteScopes("github", 1, tt.groupID, tt.pageToken)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Children) != tt.wantChildren {
				t.Errorf("len(Children) = %d, want %d", len(result.Children), tt.wantChildren)
			}
			if result.NextPageToken != tt.wantNextToken {
				t.Errorf("NextPageToken = %q, want %q", result.NextPageToken, tt.wantNextToken)
			}
		})
	}
}

// TestSearchRemoteScopes tests the SearchRemoteScopes client method.
func TestSearchRemoteScopes(t *testing.T) {
	tests := []struct {
		name         string
		search       string
		page         int
		pageSize     int
		statusCode   int
		body         string
		wantErr      bool
		wantChildren int
	}{
		{
			name:         "success with search term",
			search:       "my-repo",
			page:         1,
			pageSize:     20,
			statusCode:   http.StatusOK,
			body:         `{"children": [{"type": "scope", "id": "42", "name": "my-repo", "fullName": "org/my-repo"}]}`,
			wantChildren: 1,
		},
		{
			name:         "success no params",
			statusCode:   http.StatusOK,
			body:         `{"children": []}`,
			wantChildren: 0,
		},
		{
			name:       "not found",
			search:     "nothing",
			statusCode: http.StatusNotFound,
			body:       `{"error": "not found"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				expectedPath := "/plugins/gitlab/connections/2/search-remote-scopes"
				if r.URL.Path != expectedPath {
					t.Errorf("path = %s, want %s", r.URL.Path, expectedPath)
				}
				if tt.search != "" && r.URL.Query().Get("search") != tt.search {
					t.Errorf("search = %q, want %q", r.URL.Query().Get("search"), tt.search)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			result, err := client.SearchRemoteScopes("gitlab", 2, tt.search, tt.page, tt.pageSize)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Children) != tt.wantChildren {
				t.Errorf("len(Children) = %d, want %d", len(result.Children), tt.wantChildren)
			}
		})
	}
}
