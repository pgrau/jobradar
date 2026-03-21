package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	embedderv1 "github.com/pgrau/jobradar/proto/embedder/v1"
	"github.com/pgrau/jobradar/services/embedder/internal/cache"
	"github.com/pgrau/jobradar/services/embedder/internal/litellm"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	tracerName      = "embedder.handler"
	meterName       = "embedder"
	defaultCacheTTL = 24 * time.Hour
)

// EmbedderHandler implements embedderv1.EmbedderServiceServer.
type EmbedderHandler struct {
	embedderv1.UnimplementedEmbedderServiceServer

	litellm *litellm.Client
	cache   cache.Cache
	logger  *slog.Logger

	// metrics
	embedCounter    metric.Int64Counter
	embedLatency    metric.Float64Histogram
	cacheHitCounter metric.Int64Counter
}

// NewEmbedderHandler creates a new EmbedderHandler with OTel metrics registered.
func NewEmbedderHandler(
	litellmClient *litellm.Client,
	c cache.Cache,
	logger *slog.Logger,
) *EmbedderHandler {
	meter := otel.GetMeterProvider().Meter(meterName)

	embedCounter, _ := meter.Int64Counter("embedder.embed.total",
		metric.WithDescription("Total number of embedding requests"),
	)
	embedLatency, _ := meter.Float64Histogram("embedder.embed.latency_ms",
		metric.WithDescription("Embedding generation latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	cacheHitCounter, _ := meter.Int64Counter("embedder.cache.hits",
		metric.WithDescription("Total number of embedding cache hits"),
	)

	return &EmbedderHandler{
		litellm:         litellmClient,
		cache:           c,
		logger:          logger,
		embedCounter:    embedCounter,
		embedLatency:    embedLatency,
		cacheHitCounter: cacheHitCounter,
	}
}

// EmbedText implements EmbedderService.EmbedText.
func (h *EmbedderHandler) EmbedText(
	ctx context.Context,
	req *embedderv1.EmbedTextRequest,
) (*embedderv1.EmbedTextResponse, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "EmbedText")
	defer span.End()

	if req.GetText() == "" {
		return nil, status.Error(codes.InvalidArgument, "text must not be empty")
	}

	purpose := protoToLiteLLMPurpose(req.GetPurpose())
	cacheKey := embedCacheKey(req.GetText(), purpose)

	// --- Cache lookup ---
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
		h.cacheHitCounter.Add(ctx, 1)
		span.SetAttributes(attribute.Bool("cache.hit", true))
		return cachedToResponse(cached)
	} else if !errors.Is(err, cache.ErrCacheMiss) {
		h.logger.WarnContext(ctx, "cache get error — proceeding without cache",
			"key", cacheKey,
			"error", err,
		)
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))

	// --- Generate embedding ---
	result, err := h.litellm.EmbedText(ctx, req.GetText(), purpose)
	if err != nil {
		h.embedCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		return nil, status.Errorf(codes.Internal, "generating embedding: %v", err)
	}

	h.embedCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
	h.embedLatency.Record(ctx, float64(result.LatencyMS))

	span.SetAttributes(
		attribute.String("model", result.Model),
		attribute.Int("tokens", result.Tokens),
		attribute.Int("dimensions", len(result.Embedding)),
	)

	resp := &embedderv1.EmbedTextResponse{
		Embedding: result.Embedding,
		Model:     result.Model,
		Tokens:    int32(result.Tokens),
		LatencyMs: result.LatencyMS,
	}

	// --- Store in cache (best-effort, non-blocking) ---
	go h.storeInCache(cacheKey, resp)

	return resp, nil
}

// EmbedBatch implements EmbedderService.EmbedBatch.
func (h *EmbedderHandler) EmbedBatch(
	ctx context.Context,
	req *embedderv1.EmbedBatchRequest,
) (*embedderv1.EmbedBatchResponse, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "EmbedBatch")
	defer span.End()

	if len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "items must not be empty")
	}

	span.SetAttributes(attribute.Int("batch.size", len(req.GetItems())))

	results := make([]*embedderv1.EmbedResult, len(req.GetItems()))
	uncachedIndexes := make([]int, 0, len(req.GetItems()))
	uncachedTexts := make([]string, 0, len(req.GetItems()))

	// --- Check cache for each item ---
	for i, item := range req.GetItems() {
		purpose := protoToLiteLLMPurpose(item.GetPurpose())
		cacheKey := embedCacheKey(item.GetText(), purpose)

		if cached, err := h.cache.Get(ctx, cacheKey); err == nil {
			resp, err := cachedToResponse(cached)
			if err == nil {
				h.cacheHitCounter.Add(ctx, 1)
				results[i] = &embedderv1.EmbedResult{
					Id:        item.GetId(),
					Embedding: resp.Embedding,
					Cached:    true,
				}
				continue
			}
		}

		uncachedIndexes = append(uncachedIndexes, i)
		uncachedTexts = append(uncachedTexts, item.GetText())
	}

	// --- Generate embeddings for uncached items ---
	if len(uncachedTexts) > 0 {
		// Use purpose from first uncached item — batch assumes homogeneous purpose
		purpose := protoToLiteLLMPurpose(req.GetItems()[uncachedIndexes[0]].GetPurpose())

		batchResults, err := h.litellm.EmbedBatch(ctx, uncachedTexts, purpose)
		if err != nil {
			h.embedCounter.Add(ctx, int64(len(uncachedTexts)),
				metric.WithAttributes(attribute.String("status", "error")),
			)
			return nil, status.Errorf(codes.Internal, "generating batch embeddings: %v", err)
		}

		h.embedCounter.Add(ctx, int64(len(uncachedTexts)),
			metric.WithAttributes(attribute.String("status", "ok")),
		)

		for j, idx := range uncachedIndexes {
			item := req.GetItems()[idx]
			results[idx] = &embedderv1.EmbedResult{
				Id:        item.GetId(),
				Embedding: batchResults[j].Embedding,
				Cached:    false,
			}

			// Store in cache best-effort
			purpose := protoToLiteLLMPurpose(item.GetPurpose())
			cacheKey := embedCacheKey(item.GetText(), purpose)
			resp := &embedderv1.EmbedTextResponse{
				Embedding: batchResults[j].Embedding,
				Model:     batchResults[j].Model,
				Tokens:    int32(batchResults[j].Tokens),
			}
			go h.storeInCache(cacheKey, resp)
		}
	}

	model := ""
	if len(req.GetItems()) > 0 && results[0] != nil {
		model = "mxbai-embed-large"
	}

	return &embedderv1.EmbedBatchResponse{
		Results: results,
		Model:   model,
	}, nil
}

// EmbedCV implements EmbedderService.EmbedCV.
func (h *EmbedderHandler) EmbedCV(
	ctx context.Context,
	req *embedderv1.EmbedCVRequest,
) (*embedderv1.EmbedCVResponse, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "EmbedCV")
	defer span.End()

	if req.GetProfileId() == "" {
		return nil, status.Error(codes.InvalidArgument, "profile_id must not be empty")
	}
	if req.GetCvText() == "" {
		return nil, status.Error(codes.InvalidArgument, "cv_text must not be empty")
	}

	span.SetAttributes(attribute.String("profile_id", req.GetProfileId()))

	result, err := h.litellm.EmbedText(ctx, req.GetCvText(), litellm.PurposeDocument)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generating CV embedding: %v", err)
	}

	h.embedCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
	h.embedLatency.Record(ctx, float64(result.LatencyMS))

	span.SetAttributes(
		attribute.String("model", result.Model),
		attribute.Int("tokens", result.Tokens),
	)

	return &embedderv1.EmbedCVResponse{
		ProfileId: req.GetProfileId(),
		Embedding: result.Embedding,
		Tokens:    int32(result.Tokens),
		Model:     result.Model,
	}, nil
}

// --- helpers ---

// embedCacheKey generates a deterministic cache key from text and purpose.
func embedCacheKey(text string, purpose litellm.EmbedPurpose) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d:%s", purpose, text)))
	return fmt.Sprintf("embed:%x", h.Sum(nil))
}

// storeInCache serialises a response and stores it in cache best-effort.
// Runs in a goroutine — errors are logged but not propagated to the caller.
func (h *EmbedderHandler) storeInCache(key string, resp *embedderv1.EmbedTextResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		h.logger.Warn("failed to marshal embedding for cache", "key", key, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.cache.Set(ctx, key, data, defaultCacheTTL); err != nil {
		h.logger.Warn("failed to store embedding in cache", "key", key, "error", err)
	}
}

// cachedToResponse deserialises a cached embedding response.
func cachedToResponse(data []byte) (*embedderv1.EmbedTextResponse, error) {
	resp := &embedderv1.EmbedTextResponse{}
	if err := json.Unmarshal(data, resp); err != nil {
		return nil, fmt.Errorf("unmarshalling cached embedding: %w", err)
	}
	return resp, nil
}

// protoToLiteLLMPurpose converts the proto enum to the litellm package enum.
func protoToLiteLLMPurpose(p embedderv1.EmbedPurpose) litellm.EmbedPurpose {
	if p == embedderv1.EmbedPurpose_EMBED_PURPOSE_QUERY {
		return litellm.PurposeQuery
	}
	return litellm.PurposeDocument
}
