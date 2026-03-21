package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Cache defines the interface for embedding cache operations.
// Keeping it as an interface allows easy mocking in tests.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Close() error
}

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = fmt.Errorf("cache miss")

// ValkeyCache implements Cache backed by Valkey.
type ValkeyCache struct {
	client valkey.Client
	logger *slog.Logger
}

// NewValkey creates a new ValkeyCache and verifies connectivity.
// Returns an error if the connection cannot be established within ctx deadline.
func NewValkey(ctx context.Context, addr, password string, logger *slog.Logger) (*ValkeyCache, error) {
	opts := valkey.ClientOption{
		InitAddress: []string{addr},
	}

	if password != "" {
		opts.Password = password
	}

	client, err := valkey.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("creating valkey client: %w", err)
	}

	// Verify connectivity within the startup context deadline.
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("pinging valkey at %s: %w", addr, err)
	}

	logger.Info("valkey connected", "addr", addr)

	return &ValkeyCache{
		client: client,
		logger: logger,
	}, nil
}

// Get retrieves a cached value by key.
// Returns ErrCacheMiss if the key does not exist or has expired.
func (c *ValkeyCache) Get(ctx context.Context, key string) ([]byte, error) {
	result := c.client.Do(ctx, c.client.B().Get().Key(key).Build())

	if result.Error() != nil {
		if valkey.IsValkeyNil(result.Error()) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("getting key %q from valkey: %w", key, result.Error())
	}

	bytes, err := result.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("reading value for key %q: %w", key, err)
	}

	return bytes, nil
}

// Set stores a value with a TTL.
// A zero TTL means the key never expires — use with caution.
func (c *ValkeyCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var cmd valkey.Completed

	if ttl > 0 {
		cmd = c.client.B().Set().Key(key).Value(valkey.BinaryString(value)).
			Px(ttl).Build()
	} else {
		cmd = c.client.B().Set().Key(key).Value(valkey.BinaryString(value)).
			Build()
	}

	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("setting key %q in valkey: %w", key, err)
	}

	return nil
}

// Close releases the Valkey client connection.
func (c *ValkeyCache) Close() error {
	c.client.Close()
	return nil
}
