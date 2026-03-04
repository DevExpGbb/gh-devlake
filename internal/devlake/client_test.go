package devlake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDoGet tests the doGet generic helper.
func TestDoGet(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantID     int
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"id": 123, "name": "test"}`,
			wantID:     123,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"error": "not found"}`,
			wantErr:    true,
		},
		{
			name:       "malformed JSON",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			result, err := doGet[Connection](client, "/test")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", result.ID, tt.wantID)
			}
		})
	}
}

// TestDoPost tests the doPost generic helper.
func TestDoPost(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantID     int
	}{
		{
			name:       "success with 200",
			statusCode: http.StatusOK,
			body:       `{"id": 456}`,
			wantID:     456,
		},
		{
			name:       "success with 201",
			statusCode: http.StatusCreated,
			body:       `{"id": 789}`,
			wantID:     789,
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"error": "invalid"}`,
			wantErr:    true,
		},
		{
			name:       "malformed JSON",
			statusCode: http.StatusOK,
			body:       `{invalid}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			payload := map[string]string{"test": "data"}
			result, err := doPost[Connection](client, "/test", payload)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", result.ID, tt.wantID)
			}
		})
	}
}

// TestDoPut tests the doPut generic helper.
func TestDoPut(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantID     int
	}{
		{
			name:       "success with 200",
			statusCode: http.StatusOK,
			body:       `{"id": 111}`,
			wantID:     111,
		},
		{
			name:       "success with 201",
			statusCode: http.StatusCreated,
			body:       `{"id": 222}`,
			wantID:     222,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": "internal"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("method = %s, want PUT", r.Method)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			payload := map[string]string{"test": "data"}
			result, err := doPut[Connection](client, "/test", payload)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", result.ID, tt.wantID)
			}
		})
	}
}

// TestDoPatch tests the doPatch generic helper.
func TestDoPatch(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantID     int
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"id": 333}`,
			wantID:     333,
		},
		{
			name:       "conflict",
			statusCode: http.StatusConflict,
			body:       `{"error": "conflict"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					t.Errorf("method = %s, want PATCH", r.Method)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			payload := map[string]string{"test": "data"}
			result, err := doPatch[Connection](client, "/test", payload)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", result.ID, tt.wantID)
			}
		})
	}
}

// TestListConnections tests the ListConnections method.
func TestListConnections(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantCount  int
	}{
		{
			name:       "success with connections",
			statusCode: http.StatusOK,
			body:       `[{"id": 1, "name": "conn1"}, {"id": 2, "name": "conn2"}]`,
			wantCount:  2,
		},
		{
			name:       "empty list",
			statusCode: http.StatusOK,
			body:       `[]`,
			wantCount:  0,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error": "internal"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/plugins/github/connections" {
					t.Errorf("path = %s, want /plugins/github/connections", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			result, err := client.ListConnections("github")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("count = %d, want %d", len(result), tt.wantCount)
			}
		})
	}
}

// TestFindConnectionByName tests the FindConnectionByName method.
func TestFindConnectionByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/plugins/github/connections" {
			t.Errorf("expected path /plugins/github/connections, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": 1, "name": "conn1"}, {"id": 2, "name": "conn2"}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)

	tests := []struct {
		name     string
		findName string
		wantID   int
		wantNil  bool
	}{
		{
			name:     "found",
			findName: "conn2",
			wantID:   2,
		},
		{
			name:     "not found",
			findName: "conn3",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.FindConnectionByName("github", tt.findName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected connection, got nil")
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", result.ID, tt.wantID)
			}
		})
	}
}

// TestCreateConnection tests the CreateConnection method.
func TestCreateConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plugins/github/connections" {
			t.Errorf("path = %s, want /plugins/github/connections", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 42, "name": "new-conn"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	req := &ConnectionCreateRequest{
		Name:             "new-conn",
		Endpoint:         "https://api.github.com",
		AuthMethod:       "AccessToken",
		Token:            "ghp_test",
		RateLimitPerHour: 5000,
	}

	result, err := client.CreateConnection("github", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 42 {
		t.Errorf("ID = %d, want 42", result.ID)
	}
	if result.Name != "new-conn" {
		t.Errorf("Name = %q, want %q", result.Name, "new-conn")
	}
}

// TestDeleteConnection tests the DeleteConnection method.
func TestDeleteConnection(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success with 200",
			statusCode: http.StatusOK,
		},
		{
			name:       "success with 204",
			statusCode: http.StatusNoContent,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				if r.URL.Path != "/plugins/github/connections/123" {
					t.Errorf("path = %s, want /plugins/github/connections/123", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			err := client.DeleteConnection("github", 123)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestTestConnection tests the TestConnection method.
func TestTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plugins/github/test" {
			t.Errorf("path = %s, want /plugins/github/test", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "message": "ok"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	req := &ConnectionTestRequest{
		Name:             "test",
		Endpoint:         "https://api.github.com",
		AuthMethod:       "AccessToken",
		Token:            "ghp_test",
		RateLimitPerHour: 5000,
	}

	result, err := client.TestConnection("github", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("Success = false, want true")
	}
	if result.Message != "ok" {
		t.Errorf("Message = %q, want %q", result.Message, "ok")
	}
}

// TestGetConnection tests the GetConnection method.
func TestGetConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plugins/github/connections/99" {
			t.Errorf("path = %s, want /plugins/github/connections/99", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 99, "name": "conn99"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.GetConnection("github", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 99 {
		t.Errorf("ID = %d, want 99", result.ID)
	}
}

// TestUpdateConnection tests the UpdateConnection method.
func TestUpdateConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/plugins/github/connections/55" {
			t.Errorf("path = %s, want /plugins/github/connections/55", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 55, "name": "updated"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	req := &ConnectionUpdateRequest{
		Name: "updated",
	}
	result, err := client.UpdateConnection("github", 55, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "updated" {
		t.Errorf("Name = %q, want %q", result.Name, "updated")
	}
}

// TestPutScopes tests the PutScopes method.
func TestPutScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/plugins/github/connections/1/scopes" {
			t.Errorf("path = %s, want /plugins/github/connections/1/scopes", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	req := &ScopeBatchRequest{
		Data: []any{map[string]any{"name": "repo1"}},
	}
	err := client.PutScopes("github", 1, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestListScopes tests the ListScopes method.
func TestListScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		expectedURI := "/plugins/github/connections/1/scopes?pageSize=100&page=1"
		if r.URL.RequestURI() != expectedURI {
			t.Errorf("request URI = %s, want %s", r.URL.RequestURI(), expectedURI)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"scopes": [{"scope": {"githubId": 1, "name": "repo1"}}], "count": 1}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.ListScopes("github", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.Scopes) != 1 {
		t.Fatalf("len(Scopes) = %d, want 1", len(result.Scopes))
	}
	if result.Scopes[0].Scope.Name != "repo1" {
		t.Errorf("Name = %q, want %q", result.Scopes[0].Scope.Name, "repo1")
	}
}

// TestDeleteScope tests the DeleteScope method.
func TestDeleteScope(t *testing.T) {
	tests := []struct {
		name       string
		scopeID    string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "simple scope ID",
			scopeID:    "123",
			statusCode: http.StatusOK,
		},
		{
			name:       "scope ID with slash (URL escaped)",
			scopeID:    "org/repo",
			statusCode: http.StatusOK,
		},
		{
			name:       "not found",
			scopeID:    "999",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestURI string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				requestURI = r.RequestURI
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			err := client.DeleteScope("github", 1, tt.scopeID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			// Verify the RequestURI contains URL-escaped scope ID
			if !tt.wantErr && requestURI != "" {
				if !strings.Contains(requestURI, "/plugins/github/connections/1/scopes/") {
					t.Errorf("RequestURI %s doesn't contain expected prefix", requestURI)
				}
				// For the slash test, verify URL escaping
				if tt.scopeID == "org/repo" && !strings.Contains(requestURI, "org%2Frepo") {
					t.Errorf("RequestURI %s doesn't contain URL-escaped scope ID (expected org%%2Frepo)", requestURI)
				}
			}
		})
	}
}

// TestListProjects tests the ListProjects method.
func TestListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			t.Errorf("path = %s, want /projects", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"count": 2, "projects": [{"name": "p1"}, {"name": "p2"}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("len = %d, want 2", len(result))
	}
	if result[0].Name != "p1" {
		t.Errorf("Name = %q, want %q", result[0].Name, "p1")
	}
}

// TestCreateProject tests the CreateProject method.
func TestCreateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			t.Errorf("path = %s, want /projects", r.URL.Path)
		}
		var req Project
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"name": "new-project", "blueprint": {"id": 1}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	project := &Project{Name: "new-project"}
	result, err := client.CreateProject(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "new-project" {
		t.Errorf("Name = %q, want %q", result.Name, "new-project")
	}
	if result.Blueprint == nil || result.Blueprint.ID != 1 {
		t.Error("expected blueprint ID 1")
	}
}

// TestDeleteProject tests the DeleteProject method.
func TestDeleteProject(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("method = %s, want DELETE", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			err := client.DeleteProject("test-project")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestPatchBlueprint tests the PatchBlueprint method.
func TestPatchBlueprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/blueprints/10" {
			t.Errorf("path = %s, want /blueprints/10", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 10, "name": "updated-bp"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	patch := &BlueprintPatch{CronConfig: "0 0 * * *"}
	result, err := client.PatchBlueprint(10, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 10 {
		t.Errorf("ID = %d, want 10", result.ID)
	}
}

// TestTriggerBlueprint tests the TriggerBlueprint method.
func TestTriggerBlueprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blueprints/5/trigger" {
			t.Errorf("path = %s, want /blueprints/5/trigger", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 100, "status": "TASK_CREATED"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.TriggerBlueprint(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 100 {
		t.Errorf("ID = %d, want 100", result.ID)
	}
	if result.Status != "TASK_CREATED" {
		t.Errorf("Status = %q, want %q", result.Status, "TASK_CREATED")
	}
}

// TestPing tests the Ping method.
func TestPing(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
		},
		{
			name:       "unavailable",
			statusCode: http.StatusServiceUnavailable,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/ping" {
					t.Errorf("path = %s, want /ping", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			err := client.Ping()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestHealth tests the Health method.
func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantStatus string
		wantErr    bool
	}{
		{
			name:       "success with JSON",
			statusCode: http.StatusOK,
			body:       `{"status": "healthy"}`,
			wantStatus: "healthy",
		},
		{
			name:       "success with empty JSON",
			statusCode: http.StatusOK,
			body:       `{}`,
			wantStatus: "ok",
		},
		{
			name:       "error",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"status": "unhealthy"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			result, err := client.Health()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
		})
	}
}

// TestTestSavedConnection tests the TestSavedConnection method.
func TestTestSavedConnection(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantResult bool
		wantErr    bool
	}{
		{
			name:       "success with JSON",
			statusCode: http.StatusOK,
			body:       `{"success": true, "message": "ok"}`,
			wantResult: true,
		},
		{
			name:       "success with non-JSON (200)",
			statusCode: http.StatusOK,
			body:       `pong`,
			wantResult: true,
		},
		{
			name:       "failure with non-JSON",
			statusCode: http.StatusBadRequest,
			body:       `invalid token`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/plugins/github/connections/1/test" {
					t.Errorf("path = %s, want /plugins/github/connections/1/test", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL)
			result, err := client.TestSavedConnection("github", 1)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success != tt.wantResult {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantResult)
			}
		})
	}
}
