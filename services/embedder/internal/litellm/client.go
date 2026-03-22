package litellm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	httpTimeout  = 30 * time.Second
	maxRetries   = 3
	defaultModel = "mxbai-embed-large"
)

// Client wraps the OpenAI-compatible LiteLLM API for embedding generation.
// LiteLLM exposes an OpenAI-compatible endpoint — this client works
// transparently with Ollama (local) and Gemini (production) via LiteLLM routing.
type Client struct {
	openai *openai.Client
	model  string
	logger *slog.Logger
}

// EmbedResult holds the result of a single embedding generation.
type EmbedResult struct {
	Embedding []float32
	Model     string
	Tokens    int
	LatencyMS int64
}

// NewClient creates a LiteLLM client and verifies the endpoint is reachable.
func NewClient(ctx context.Context, baseURL, apiKey string, logger *slog.Logger) (*Client, error) {
	httpClient := &http.Client{
		Timeout: httpTimeout,
	}

	c := openai.NewClient(
		option.WithBaseURL(baseURL+"/v1"),
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(maxRetries),
	)

	client := &Client{
		openai: &c,
		model:  defaultModel,
		logger: logger,
	}

	if err := client.ping(ctx); err != nil {
		return nil, fmt.Errorf("litellm not reachable at %s: %w", baseURL, err)
	}

	logger.Info("litellm connected", "base_url", baseURL, "model", defaultModel)

	return client, nil
}

// EmbedText generates an embedding for a single text input.
func (c *Client) EmbedText(ctx context.Context, text string, purpose EmbedPurpose) (*EmbedResult, error) {
	if text == "" {
		return nil, fmt.Errorf("text must not be empty")
	}

	input := ApplyPurposePrefix(text, purpose)
	start := time.Now()

	resp, err := c.openai.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(c.model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(input),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("litellm returned empty embedding data")
	}

	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return &EmbedResult{
		Embedding: embedding,
		Model:     resp.Model,
		Tokens:    int(resp.Usage.TotalTokens),
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}

// EmbedBatch generates embeddings for multiple texts in a single API call.
func (c *Client) EmbedBatch(ctx context.Context, texts []string, purpose EmbedPurpose) ([]*EmbedResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts must not be empty")
	}

	inputs := make([]string, len(texts))
	for i, text := range texts {
		inputs[i] = ApplyPurposePrefix(text, purpose)
	}

	start := time.Now()

	resp, err := c.openai.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(c.model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: inputs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("generating batch embeddings: %w", err)
	}

	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(resp.Data))
	}

	latencyMS := time.Since(start).Milliseconds()
	results := make([]*EmbedResult, len(resp.Data))

	for i, data := range resp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		results[i] = &EmbedResult{
			Embedding: embedding,
			Model:     resp.Model,
			Tokens:    int(resp.Usage.TotalTokens) / len(texts),
			LatencyMS: latencyMS,
		}
	}

	return results, nil
}

// EmbedPurpose controls task-specific prefixes for asymmetric embedding models.
type EmbedPurpose int

const (
	PurposeDocument EmbedPurpose = iota
	PurposeQuery
)

// ApplyPurposePrefix applies task-specific prefixes for mxbai-embed-large.
// Exported as a pure function — used internally by Client and directly in tests.
func ApplyPurposePrefix(text string, purpose EmbedPurpose) string {
	if purpose == PurposeQuery {
		return "Represent this sentence for searching relevant passages: " + text
	}
	return text
}

// ping verifies LiteLLM is reachable.
func (c *Client) ping(ctx context.Context) error {
	_, err := c.EmbedText(ctx, "ping", PurposeDocument)
	return err
}
