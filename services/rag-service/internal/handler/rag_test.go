package handler_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	ragv1 "github.com/pgrau/jobradar/proto/rag/v1"
	"github.com/pgrau/jobradar/services/rag-service/internal/db"
	"github.com/pgrau/jobradar/services/rag-service/internal/handler"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- mock ---

type mockRepo struct {
	searchOffersFn    func(ctx context.Context, params db.SearchParams) ([]*db.SearchResult, int, error)
	getSimilarFn      func(ctx context.Context, profileID, excludeOfferID string, embedding []float32, limit, daysAgo int32) ([]*db.SearchResult, error)
	storeOfferFn      func(ctx context.Context, offerID string, embedding []float32) error
	getMarketContextFn func(ctx context.Context, profileID, role, region, topic string, daysAgo, maxOffers int32) ([]*db.SearchResult, int, error)
}

func (m *mockRepo) SearchOffers(ctx context.Context, params db.SearchParams) ([]*db.SearchResult, int, error) {
	return m.searchOffersFn(ctx, params)
}
func (m *mockRepo) GetSimilarOffers(ctx context.Context, profileID, excludeOfferID string, embedding []float32, limit, daysAgo int32) ([]*db.SearchResult, error) {
	return m.getSimilarFn(ctx, profileID, excludeOfferID, embedding, limit, daysAgo)
}
func (m *mockRepo) StoreOffer(ctx context.Context, offerID string, embedding []float32) error {
	return m.storeOfferFn(ctx, offerID, embedding)
}
func (m *mockRepo) GetMarketContext(ctx context.Context, profileID, role, region, topic string, daysAgo, maxOffers int32) ([]*db.SearchResult, int, error) {
	return m.getMarketContextFn(ctx, profileID, role, region, topic, daysAgo, maxOffers)
}

// --- helpers ---

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func fakeResult() *db.SearchResult {
	now := time.Now()
	return &db.SearchResult{
		OfferID:      "offer-123",
		ProfileID:    "profile-456",
		Title:        "Staff Backend Engineer",
		Company:      "Acme",
		Location:     "Barcelona",
		Source:       "linkedin",
		URL:          "https://linkedin.com/jobs/123",
		Score:        87.5,
		Similarity:   0.91,
		Reasoning:    "Strong match: Go + Kubernetes",
		SkillMatches: []string{"Go", "Kubernetes"},
		SkillGaps:    []string{"Terraform"},
		Reviewed:     false,
		Saved:        false,
		ScoredAt:     now,
		PostedAt:     &now,
	}
}

func fakeEmbedding() []float32 {
	v := make([]float32, 1024)
	for i := range v {
		v[i] = 0.01 * float32(i%100)
	}
	return v
}

// --- SearchOffers tests ---

func TestSearchOffers_HappyPath(t *testing.T) {
	repo := &mockRepo{
		searchOffersFn: func(_ context.Context, _ db.SearchParams) ([]*db.SearchResult, int, error) {
			return []*db.SearchResult{fakeResult()}, 1, nil
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	resp, err := h.SearchOffers(context.Background(), &ragv1.SearchOffersRequest{
		ProfileId:      "profile-456",
		QueryEmbedding: fakeEmbedding(),
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
	if resp.Results[0].OfferId != "offer-123" {
		t.Errorf("unexpected offer_id: %s", resp.Results[0].OfferId)
	}
}

func TestSearchOffers_MissingProfileID_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())

	_, err := h.SearchOffers(context.Background(), &ragv1.SearchOffersRequest{
		Query: "Go engineer",
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestSearchOffers_MissingQueryAndEmbedding_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())

	_, err := h.SearchOffers(context.Background(), &ragv1.SearchOffersRequest{
		ProfileId: "profile-456",
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestSearchOffers_WrongEmbeddingDimensions_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.SearchOffers(context.Background(), &ragv1.SearchOffersRequest{
		ProfileId:      "profile-456",
		QueryEmbedding: []float32{0.1, 0.2, 0.3}, // wrong: only 3 dims
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestSearchOffers_RepoError_ReturnsInternal(t *testing.T) {
	repo := &mockRepo{
		searchOffersFn: func(_ context.Context, _ db.SearchParams) ([]*db.SearchResult, int, error) {
			return nil, 0, errors.New("connection lost")
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	_, err := h.SearchOffers(context.Background(), &ragv1.SearchOffersRequest{
		ProfileId: "profile-456",
		Query:     "Go engineer",
	})
	assertGRPCCode(t, err, codes.Internal)
}

func TestSearchOffers_FiltersArePropagated(t *testing.T) {
	var capturedParams db.SearchParams
	repo := &mockRepo{
		searchOffersFn: func(_ context.Context, params db.SearchParams) ([]*db.SearchResult, int, error) {
			capturedParams = params
			return nil, 0, nil
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	_, err := h.SearchOffers(context.Background(), &ragv1.SearchOffersRequest{
		ProfileId: "profile-456",
		Query:     "Go engineer",
		Filters: &ragv1.SearchFilters{
			Locations:  []string{"Barcelona"},
			MinScore:   75,
			RemoteOnly: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedParams.MinScore != 75 {
		t.Errorf("expected MinScore=75, got %d", capturedParams.MinScore)
	}
	if !capturedParams.RemoteOnly {
		t.Error("expected RemoteOnly=true")
	}
	if len(capturedParams.Locations) != 1 || capturedParams.Locations[0] != "Barcelona" {
		t.Errorf("unexpected locations: %v", capturedParams.Locations)
	}
}

// --- GetSimilarOffers tests ---

func TestGetSimilarOffers_HappyPath(t *testing.T) {
	repo := &mockRepo{
		getSimilarFn: func(_ context.Context, _, _ string, _ []float32, _, _ int32) ([]*db.SearchResult, error) {
			return []*db.SearchResult{fakeResult(), fakeResult()}, nil
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	resp, err := h.GetSimilarOffers(context.Background(), &ragv1.GetSimilarOffersRequest{
		ProfileId: "profile-456",
		OfferId:   "offer-123",
		Embedding: fakeEmbedding(),
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
}

func TestGetSimilarOffers_MissingProfileID_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.GetSimilarOffers(context.Background(), &ragv1.GetSimilarOffersRequest{
		OfferId:   "offer-123",
		Embedding: fakeEmbedding(),
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestGetSimilarOffers_MissingOfferID_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.GetSimilarOffers(context.Background(), &ragv1.GetSimilarOffersRequest{
		ProfileId: "profile-456",
		Embedding: fakeEmbedding(),
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestGetSimilarOffers_MissingEmbedding_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.GetSimilarOffers(context.Background(), &ragv1.GetSimilarOffersRequest{
		ProfileId: "profile-456",
		OfferId:   "offer-123",
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestGetSimilarOffers_WrongDimensions_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.GetSimilarOffers(context.Background(), &ragv1.GetSimilarOffersRequest{
		ProfileId: "profile-456",
		OfferId:   "offer-123",
		Embedding: []float32{0.1, 0.2}, // wrong: only 2 dims
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

// --- StoreOffer tests ---

func TestStoreOffer_HappyPath(t *testing.T) {
	repo := &mockRepo{
		storeOfferFn: func(_ context.Context, _ string, _ []float32) error {
			return nil
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	resp, err := h.StoreOffer(context.Background(), &ragv1.StoreOfferRequest{
		ProfileId: "profile-456",
		OfferId:   "offer-123",
		Embedding: fakeEmbedding(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Stored {
		t.Error("expected stored=true")
	}
	if resp.OfferId != "offer-123" {
		t.Errorf("unexpected offer_id: %s", resp.OfferId)
	}
}

func TestStoreOffer_MissingProfileID_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.StoreOffer(context.Background(), &ragv1.StoreOfferRequest{
		OfferId:   "offer-123",
		Embedding: fakeEmbedding(),
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestStoreOffer_MissingOfferID_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.StoreOffer(context.Background(), &ragv1.StoreOfferRequest{
		ProfileId: "profile-456",
		Embedding: fakeEmbedding(),
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestStoreOffer_WrongDimensions_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.StoreOffer(context.Background(), &ragv1.StoreOfferRequest{
		OfferId:   "offer-123",
		Embedding: []float32{0.1, 0.2, 0.3}, // wrong: only 3 dims
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestStoreOffer_OfferNotFound_ReturnsNotFound(t *testing.T) {
	repo := &mockRepo{
		storeOfferFn: func(_ context.Context, _ string, _ []float32) error {
			return db.ErrOfferNotFound
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	_, err := h.StoreOffer(context.Background(), &ragv1.StoreOfferRequest{
		ProfileId: "profile-456",
		OfferId:   "nonexistent",
		Embedding: fakeEmbedding(),
	})
	assertGRPCCode(t, err, codes.NotFound)
}

func TestStoreOffer_RepoError_ReturnsInternal(t *testing.T) {
	repo := &mockRepo{
		storeOfferFn: func(_ context.Context, _ string, _ []float32) error {
			return errors.New("db connection lost")
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	_, err := h.StoreOffer(context.Background(), &ragv1.StoreOfferRequest{
		ProfileId: "profile-456",
		OfferId:   "offer-123",
		Embedding: fakeEmbedding(),
	})
	assertGRPCCode(t, err, codes.Internal)
}

// --- GetMarketContext tests ---

func TestGetMarketContext_HappyPath(t *testing.T) {
	repo := &mockRepo{
		getMarketContextFn: func(_ context.Context, _, _, _, _ string, _, _ int32) ([]*db.SearchResult, int, error) {
			return []*db.SearchResult{fakeResult()}, 42, nil
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	resp, err := h.GetMarketContext(context.Background(), &ragv1.GetMarketContextRequest{
		ProfileId: "profile-456",
		Role:      "Staff Backend Engineer",
		Topic:     "Go",
		DaysAgo:   30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ContextOffers) != 1 {
		t.Errorf("expected 1 context offer, got %d", len(resp.ContextOffers))
	}
	if resp.TotalOffers != 42 {
		t.Errorf("expected total=42, got %d", resp.TotalOffers)
	}
	if resp.Period != "last_30_days" {
		t.Errorf("expected period='last_30_days', got %q", resp.Period)
	}
}

func TestGetMarketContext_MissingProfileID_ReturnsInvalidArgument(t *testing.T) {
	h := handler.NewRAGHandler(&mockRepo{}, discardLogger())
	_, err := h.GetMarketContext(context.Background(), &ragv1.GetMarketContextRequest{
		Role: "Backend Engineer",
	})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

func TestGetMarketContext_ZeroDaysAgo_PeriodIsAllTime(t *testing.T) {
	repo := &mockRepo{
		getMarketContextFn: func(_ context.Context, _, _, _, _ string, _, _ int32) ([]*db.SearchResult, int, error) {
			return nil, 0, nil
		},
	}
	h := handler.NewRAGHandler(repo, discardLogger())

	resp, err := h.GetMarketContext(context.Background(), &ragv1.GetMarketContextRequest{
		ProfileId: "profile-456",
		DaysAgo:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Period != "all_time" {
		t.Errorf("expected period='all_time', got %q", resp.Period)
	}
}

// --- assertion helper ---

func assertGRPCCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if s.Code() != want {
		t.Errorf("expected gRPC code %s, got %s: %s", want, s.Code(), s.Message())
	}
}
