package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	ragv1 "github.com/pgrau/jobradar/proto/rag/v1"
	"github.com/pgrau/jobradar/services/rag-service/internal/db"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	tracerName = "rag.handler"
	meterName  = "rag"
)

// Repository defines the data access interface for the RAG handler.
// Defined here (consumer) so the db package satisfies it without coupling.
type Repository interface {
	SearchOffers(ctx context.Context, params db.SearchParams) ([]*db.SearchResult, int, error)
	GetSimilarOffers(ctx context.Context, profileID, excludeOfferID string, embedding []float32, limit, daysAgo int32) ([]*db.SearchResult, error)
	StoreOffer(ctx context.Context, offerID string, embedding []float32) error
	GetMarketContext(ctx context.Context, profileID, role, region, topic string, daysAgo, maxOffers int32) ([]*db.SearchResult, int, error)
}

// RAGHandler implements ragv1.RAGServiceServer.
type RAGHandler struct {
	ragv1.UnimplementedRAGServiceServer

	repo   Repository
	logger *slog.Logger

	// metrics
	searchCounter        metric.Int64Counter
	searchLatency        metric.Float64Histogram
	storeCounter         metric.Int64Counter
	marketContextCounter metric.Int64Counter
}

// NewRAGHandler creates a RAGHandler with OTel metrics registered.
func NewRAGHandler(repo Repository, logger *slog.Logger) *RAGHandler {
	meter := otel.GetMeterProvider().Meter(meterName)

	searchCounter, _ := meter.Int64Counter("rag.search.total",
		metric.WithDescription("Total number of semantic search requests"),
	)
	searchLatency, _ := meter.Float64Histogram("rag.search.latency_ms",
		metric.WithDescription("Search query latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	storeCounter, _ := meter.Int64Counter("rag.store.total",
		metric.WithDescription("Total number of offer store requests"),
	)
	marketContextCounter, _ := meter.Int64Counter("rag.market_context.total",
		metric.WithDescription("Total number of market context requests"),
	)

	return &RAGHandler{
		repo:                 repo,
		logger:               logger,
		searchCounter:        searchCounter,
		searchLatency:        searchLatency,
		storeCounter:         storeCounter,
		marketContextCounter: marketContextCounter,
	}
}

// SearchOffers performs semantic or score-based search over scored offers.
func (h *RAGHandler) SearchOffers(
	ctx context.Context,
	req *ragv1.SearchOffersRequest,
) (*ragv1.SearchOffersResponse, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "SearchOffers")
	defer span.End()

	if req.GetProfileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id must not be empty")
	}
	if req.GetQuery() == "" && len(req.GetQueryEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query or query_embedding must be provided")
	}
	if len(req.GetQueryEmbedding()) > 0 && len(req.GetQueryEmbedding()) != 1024 {
		return nil, status.Errorf(codes.InvalidArgument,
			"query_embedding must be 1024 dimensions, got %d", len(req.GetQueryEmbedding()))
	}

	span.SetAttributes(
		attribute.String("profile_id", req.GetProfileId()),
		attribute.Bool("has_embedding", len(req.GetQueryEmbedding()) > 0),
		attribute.Int("limit", int(req.GetLimit())),
	)

	start := time.Now()

	params := db.SearchParams{
		ProfileID:      req.GetProfileId(),
		QueryEmbedding: req.GetQueryEmbedding(),
		Limit:          req.GetLimit(),
		Offset:         req.GetOffset(),
	}
	if f := req.GetFilters(); f != nil {
		params.Locations = f.GetLocations()
		params.Companies = f.GetCompanies()
		params.Sources = f.GetSources()
		params.MinScore = f.GetMinScore()
		params.DaysAgo = f.GetDaysAgo()
		params.RemoteOnly = f.GetRemoteOnly()
		params.MinSalaryEUR = f.GetMinSalaryEur()
	}

	results, total, err := h.repo.SearchOffers(ctx, params)
	if err != nil {
		h.searchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		span.RecordError(err)
		return nil, status.Errorf(codes.Internal, "searching offers: %v", err)
	}

	latencyMS := float64(time.Since(start).Milliseconds())
	h.searchCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
	h.searchLatency.Record(ctx, latencyMS)
	span.SetAttributes(
		attribute.Int("results.count", len(results)),
		attribute.Int("results.total", total),
	)

	return &ragv1.SearchOffersResponse{
		Results: toProtoResults(results),
		Total:   int32(total),
	}, nil
}

// GetSimilarOffers returns offers similar to the given reference offer.
func (h *RAGHandler) GetSimilarOffers(
	ctx context.Context,
	req *ragv1.GetSimilarOffersRequest,
) (*ragv1.GetSimilarOffersResponse, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetSimilarOffers")
	defer span.End()

	if req.GetProfileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id must not be empty")
	}
	if req.GetOfferId() == "" {
		return nil, status.Error(codes.InvalidArgument, "offer_id must not be empty")
	}
	if len(req.GetEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "embedding must not be empty")
	}
	if len(req.GetEmbedding()) != 1024 {
		return nil, status.Errorf(codes.InvalidArgument,
			"embedding must be 1024 dimensions, got %d", len(req.GetEmbedding()))
	}

	span.SetAttributes(
		attribute.String("profile_id", req.GetProfileId()),
		attribute.String("offer_id", req.GetOfferId()),
	)

	results, err := h.repo.GetSimilarOffers(
		ctx,
		req.GetProfileId(),
		req.GetOfferId(),
		req.GetEmbedding(),
		req.GetLimit(),
		req.GetDaysAgo(),
	)
	if err != nil {
		span.RecordError(err)
		return nil, status.Errorf(codes.Internal, "getting similar offers: %v", err)
	}

	span.SetAttributes(attribute.Int("results.count", len(results)))

	return &ragv1.GetSimilarOffersResponse{
		Results: toProtoResults(results),
	}, nil
}

// StoreOffer persists the embedding for an already-ingested offer.
func (h *RAGHandler) StoreOffer(
	ctx context.Context,
	req *ragv1.StoreOfferRequest,
) (*ragv1.StoreOfferResponse, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "StoreOffer")
	defer span.End()

	if req.GetProfileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id must not be empty")
	}
	if req.GetOfferId() == "" {
		return nil, status.Error(codes.InvalidArgument, "offer_id must not be empty")
	}
	if len(req.GetEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "embedding must not be empty")
	}
	if len(req.GetEmbedding()) != 1024 {
		return nil, status.Errorf(codes.InvalidArgument,
			"embedding must be 1024 dimensions, got %d", len(req.GetEmbedding()))
	}

	span.SetAttributes(
		attribute.String("offer_id", req.GetOfferId()),
		attribute.Int("embedding.dimensions", len(req.GetEmbedding())),
	)

	err := h.repo.StoreOffer(ctx, req.GetOfferId(), req.GetEmbedding())
	if err != nil {
		h.storeCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		span.RecordError(err)

		if errors.Is(err, db.ErrOfferNotFound) {
			return nil, status.Errorf(codes.NotFound, "offer %s not found", req.GetOfferId())
		}
		return nil, status.Errorf(codes.Internal, "storing offer embedding: %v", err)
	}

	h.storeCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))

	return &ragv1.StoreOfferResponse{
		OfferId: req.GetOfferId(),
		Stored:  true,
	}, nil
}

// GetMarketContext returns top scored offers for LLM context grounding.
func (h *RAGHandler) GetMarketContext(
	ctx context.Context,
	req *ragv1.GetMarketContextRequest,
) (*ragv1.GetMarketContextResponse, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetMarketContext")
	defer span.End()

	if req.GetProfileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id must not be empty")
	}

	span.SetAttributes(
		attribute.String("profile_id", req.GetProfileId()),
		attribute.String("role", req.GetRole()),
		attribute.String("region", req.GetRegion()),
		attribute.String("topic", req.GetTopic()),
	)

	results, total, err := h.repo.GetMarketContext(
		ctx,
		req.GetProfileId(),
		req.GetRole(),
		req.GetRegion(),
		req.GetTopic(),
		req.GetDaysAgo(),
		req.GetMaxOffers(),
	)
	if err != nil {
		h.marketContextCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		span.RecordError(err)
		return nil, status.Errorf(codes.Internal, "getting market context: %v", err)
	}

	h.marketContextCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
	span.SetAttributes(
		attribute.Int("results.count", len(results)),
		attribute.Int("results.total", total),
	)

	daysAgo := req.GetDaysAgo()
	period := fmt.Sprintf("last_%d_days", daysAgo)
	if daysAgo == 0 {
		period = "all_time"
	}

	return &ragv1.GetMarketContextResponse{
		ContextOffers: toProtoResults(results),
		TotalOffers:   int32(total),
		Period:        period,
	}, nil
}

// --- helpers ---

func toProtoResults(results []*db.SearchResult) []*ragv1.OfferResult {
	out := make([]*ragv1.OfferResult, len(results))
	for i, r := range results {
		out[i] = &ragv1.OfferResult{
			OfferId:      r.OfferID,
			ProfileId:    r.ProfileID,
			Title:        r.Title,
			Company:      r.Company,
			Location:     r.Location,
			Source:       r.Source,
			Url:          r.URL,
			Score:        int32(r.Score),
			Similarity:   r.Similarity,
			Reasoning:    r.Reasoning,
			SkillMatches: r.SkillMatches,
			SkillGaps:    r.SkillGaps,
			Reviewed:     r.Reviewed,
			Saved:        r.Saved,
			IngestedAt:   r.ScoredAt.Unix(),
		}
		if r.PostedAt != nil {
			out[i].PostedAt = r.PostedAt.Unix()
		}
	}
	return out
}
