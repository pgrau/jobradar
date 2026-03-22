package handler_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	embedderv1 "github.com/pgrau/jobradar/proto/embedder/v1"
	"github.com/pgrau/jobradar/services/embedder/internal/cache"
	"github.com/pgrau/jobradar/services/embedder/internal/handler"
	"github.com/pgrau/jobradar/services/embedder/internal/litellm"

	"log/slog"
	"os"
)

// --- mocks ---

type mockLiteLLM struct {
	embedTextFn  func(ctx context.Context, text string, purpose litellm.EmbedPurpose) (*litellm.EmbedResult, error)
	embedBatchFn func(ctx context.Context, texts []string, purpose litellm.EmbedPurpose) ([]*litellm.EmbedResult, error)
}

func (m *mockLiteLLM) EmbedText(ctx context.Context, text string, purpose litellm.EmbedPurpose) (*litellm.EmbedResult, error) {
	return m.embedTextFn(ctx, text, purpose)
}

func (m *mockLiteLLM) EmbedBatch(ctx context.Context, texts []string, purpose litellm.EmbedPurpose) ([]*litellm.EmbedResult, error) {
	return m.embedBatchFn(ctx, texts, purpose)
}

type mockCache struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string][]byte)}
}

func (m *mockCache) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	return v, nil
}

func (m *mockCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockCache) Close() error { return nil }

// --- helpers ---

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func fakeEmbedding(dims int) []float32 {
	e := make([]float32, dims)
	for i := range e {
		e[i] = float32(i) * 0.001
	}
	return e
}

func fakeEmbedResult() *litellm.EmbedResult {
	return &litellm.EmbedResult{
		Embedding: fakeEmbedding(1024),
		Model:     "mxbai-embed-large",
		Tokens:    8,
		LatencyMS: 42,
	}
}

// --- tests ---

func TestEmbedText_Success(t *testing.T) {
	mock := &mockLiteLLM{
		embedTextFn: func(_ context.Context, text string, _ litellm.EmbedPurpose) (*litellm.EmbedResult, error) {
			return fakeEmbedResult(), nil
		},
	}

	h := handler.NewEmbedderHandler(mock, newMockCache(), testLogger())

	resp, err := h.EmbedText(context.Background(), &embedderv1.EmbedTextRequest{
		Text:    "Staff Backend Engineer Go Kubernetes",
		Purpose: embedderv1.EmbedPurpose_EMBED_PURPOSE_DOCUMENT,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Embedding) != 1024 {
		t.Errorf("expected 1024 dimensions, got %d", len(resp.Embedding))
	}
	if resp.Model != "mxbai-embed-large" {
		t.Errorf("expected model mxbai-embed-large, got %s", resp.Model)
	}
	if resp.Tokens != 8 {
		t.Errorf("expected 8 tokens, got %d", resp.Tokens)
	}
}

func TestEmbedText_EmptyText_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewEmbedderHandler(&mockLiteLLM{}, newMockCache(), testLogger())

	_, err := h.EmbedText(context.Background(), &embedderv1.EmbedTextRequest{
		Text: "",
	})

	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}
}

func TestEmbedText_CacheHit_DoesNotCallLiteLLM(t *testing.T) {
	var calls atomic.Int64
	mock := &mockLiteLLM{
		embedTextFn: func(_ context.Context, _ string, _ litellm.EmbedPurpose) (*litellm.EmbedResult, error) {
			calls.Add(1)
			return fakeEmbedResult(), nil
		},
	}

	c := newMockCache()
	h := handler.NewEmbedderHandler(mock, c, testLogger())

	req := &embedderv1.EmbedTextRequest{
		Text:    "cached text",
		Purpose: embedderv1.EmbedPurpose_EMBED_PURPOSE_DOCUMENT,
	}

	// First call — cache miss, calls LiteLLM
	_, err := h.EmbedText(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Wait for background cache write
	time.Sleep(100 * time.Millisecond)

	// Second call — should hit cache
	_, err = h.EmbedText(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("expected LiteLLM to be called once, got %d calls", calls.Load())
	}
}

func TestEmbedText_LiteLLMError_ReturnsInternalError(t *testing.T) {
	mock := &mockLiteLLM{
		embedTextFn: func(_ context.Context, _ string, _ litellm.EmbedPurpose) (*litellm.EmbedResult, error) {
			return nil, errors.New("litellm unavailable")
		},
	}

	h := handler.NewEmbedderHandler(mock, newMockCache(), testLogger())

	_, err := h.EmbedText(context.Background(), &embedderv1.EmbedTextRequest{
		Text:    "some text",
		Purpose: embedderv1.EmbedPurpose_EMBED_PURPOSE_DOCUMENT,
	})

	if err == nil {
		t.Fatal("expected error when LiteLLM fails, got nil")
	}
}

func TestEmbedBatch_Success(t *testing.T) {
	mock := &mockLiteLLM{
		embedTextFn: func(_ context.Context, _ string, _ litellm.EmbedPurpose) (*litellm.EmbedResult, error) {
			return fakeEmbedResult(), nil
		},
		embedBatchFn: func(_ context.Context, texts []string, _ litellm.EmbedPurpose) ([]*litellm.EmbedResult, error) {
			results := make([]*litellm.EmbedResult, len(texts))
			for i := range texts {
				results[i] = fakeEmbedResult()
			}
			return results, nil
		},
	}

	h := handler.NewEmbedderHandler(mock, newMockCache(), testLogger())

	resp, err := h.EmbedBatch(context.Background(), &embedderv1.EmbedBatchRequest{
		Items: []*embedderv1.EmbedItem{
			{Id: "1", Text: "Go developer", Purpose: embedderv1.EmbedPurpose_EMBED_PURPOSE_DOCUMENT},
			{Id: "2", Text: "Kubernetes engineer", Purpose: embedderv1.EmbedPurpose_EMBED_PURPOSE_DOCUMENT},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[0].Id != "1" {
		t.Errorf("expected id 1, got %s", resp.Results[0].Id)
	}
	if len(resp.Results[0].Embedding) != 1024 {
		t.Errorf("expected 1024 dimensions, got %d", len(resp.Results[0].Embedding))
	}
}

func TestEmbedBatch_EmptyItems_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewEmbedderHandler(&mockLiteLLM{}, newMockCache(), testLogger())

	_, err := h.EmbedBatch(context.Background(), &embedderv1.EmbedBatchRequest{
		Items: []*embedderv1.EmbedItem{},
	})

	if err == nil {
		t.Fatal("expected error for empty items, got nil")
	}
}

func TestEmbedCV_Success(t *testing.T) {
	mock := &mockLiteLLM{
		embedTextFn: func(_ context.Context, _ string, _ litellm.EmbedPurpose) (*litellm.EmbedResult, error) {
			return fakeEmbedResult(), nil
		},
	}

	h := handler.NewEmbedderHandler(mock, newMockCache(), testLogger())

	resp, err := h.EmbedCV(context.Background(), &embedderv1.EmbedCVRequest{
		ProfileId: "profile-001",
		CvText:    "Tech Lead 20 years Go Kubernetes DDD distributed systems",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProfileId != "profile-001" {
		t.Errorf("expected profile_id profile-001, got %s", resp.ProfileId)
	}
	if len(resp.Embedding) != 1024 {
		t.Errorf("expected 1024 dimensions, got %d", len(resp.Embedding))
	}
}

func TestEmbedCV_MissingProfileId_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewEmbedderHandler(&mockLiteLLM{}, newMockCache(), testLogger())

	_, err := h.EmbedCV(context.Background(), &embedderv1.EmbedCVRequest{
		ProfileId: "",
		CvText:    "some cv text",
	})

	if err == nil {
		t.Fatal("expected error for missing profile_id, got nil")
	}
}

func TestEmbedCV_MissingCVText_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewEmbedderHandler(&mockLiteLLM{}, newMockCache(), testLogger())

	_, err := h.EmbedCV(context.Background(), &embedderv1.EmbedCVRequest{
		ProfileId: "profile-001",
		CvText:    "",
	})

	if err == nil {
		t.Fatal("expected error for missing cv_text, got nil")
	}
}
