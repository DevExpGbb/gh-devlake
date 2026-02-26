package cmd

import (
	"testing"
)

func TestDeduplicateResults_RemovesDuplicates(t *testing.T) {
	input := []ConnSetupResult{
		{Plugin: "github", ConnectionID: 1, Name: "GitHub"},
		{Plugin: "github", ConnectionID: 1, Name: "GitHub"},
	}
	got := deduplicateResults(input)
	if len(got) != 1 {
		t.Errorf("expected 1 result, got %d", len(got))
	}
	if got[0].ConnectionID != 1 {
		t.Errorf("expected ConnectionID=1, got %d", got[0].ConnectionID)
	}
}

func TestDeduplicateResults_KeepsDifferentIDs(t *testing.T) {
	input := []ConnSetupResult{
		{Plugin: "github", ConnectionID: 1, Name: "GitHub - org1"},
		{Plugin: "github", ConnectionID: 2, Name: "GitHub - org2"},
	}
	got := deduplicateResults(input)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestDeduplicateResults_KeepsDifferentPlugins(t *testing.T) {
	input := []ConnSetupResult{
		{Plugin: "github", ConnectionID: 1, Name: "GitHub"},
		{Plugin: "gh-copilot", ConnectionID: 1, Name: "Copilot"},
	}
	got := deduplicateResults(input)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestDeduplicateResults_Empty(t *testing.T) {
	got := deduplicateResults(nil)
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestDeduplicateResults_PreservesOrder(t *testing.T) {
	input := []ConnSetupResult{
		{Plugin: "github", ConnectionID: 1, Name: "First"},
		{Plugin: "gh-copilot", ConnectionID: 1, Name: "Copilot"},
		{Plugin: "github", ConnectionID: 1, Name: "First again"},
		{Plugin: "github", ConnectionID: 2, Name: "Second GitHub"},
	}
	got := deduplicateResults(input)
	if len(got) != 3 {
		t.Errorf("expected 3 results, got %d", len(got))
	}
	if got[0].Name != "First" {
		t.Errorf("expected first result to be 'First', got %q", got[0].Name)
	}
	if got[1].Plugin != "gh-copilot" {
		t.Errorf("expected second result to be gh-copilot, got %q", got[1].Plugin)
	}
	if got[2].ConnectionID != 2 {
		t.Errorf("expected third result to have ConnectionID=2, got %d", got[2].ConnectionID)
	}
}
