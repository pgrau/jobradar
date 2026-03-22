package litellm_test

import (
	"testing"

	"github.com/pgrau/jobradar/services/embedder/internal/litellm"
)

func TestApplyPurposePrefix_Document(t *testing.T) {
	result := litellm.ApplyPurposePrefix("Staff Backend Engineer", litellm.PurposeDocument)

	if result != "Staff Backend Engineer" {
		t.Errorf("expected no prefix for document, got %q", result)
	}
}

func TestApplyPurposePrefix_Query(t *testing.T) {
	result := litellm.ApplyPurposePrefix("Staff Backend Engineer", litellm.PurposeQuery)

	expected := "Represent this sentence for searching relevant passages: Staff Backend Engineer"
	if result != expected {
		t.Errorf("expected query prefix, got %q", result)
	}
}

func TestApplyPurposePrefix_EmptyText_Document(t *testing.T) {
	result := litellm.ApplyPurposePrefix("", litellm.PurposeDocument)

	if result != "" {
		t.Errorf("expected empty string for document with empty text, got %q", result)
	}
}

func TestApplyPurposePrefix_EmptyText_Query(t *testing.T) {
	result := litellm.ApplyPurposePrefix("", litellm.PurposeQuery)

	expected := "Represent this sentence for searching relevant passages: "
	if result != expected {
		t.Errorf("expected prefix only for query with empty text, got %q", result)
	}
}
