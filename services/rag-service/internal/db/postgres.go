package db

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SearchResult holds the fields returned by pgvector similarity queries.
// All fields come from the scored_offer table — no JOINs (ADR-011).
type SearchResult struct {
	OfferID      string
	ProfileID    string
	Title        string
	Company      string
	Location     string
	Source       string
	URL          string
	Score        float32 // NUMERIC(5,2) from DB; proto maps to int32(score) — decimal truncated by design
	Similarity   float32
	Reasoning    string
	SkillMatches []string
	SkillGaps    []string
	Reviewed     bool
	Saved        bool
	ScoredAt     time.Time
	PostedAt     *time.Time
}

// SearchParams holds parameters for semantic offer search.
type SearchParams struct {
	ProfileID      string
	QueryEmbedding []float32 // optional — if empty, falls back to score ordering
	Locations      []string
	Companies      []string
	Sources        []string
	MinScore       int32
	DaysAgo        int32
	RemoteOnly     bool
	MinSalaryEUR   int32
	Limit          int32
	Offset         int32
}

// Postgres wraps a pgx connection and provides typed query methods.
// TODO: migrate to pgxpool once puddle/v2 is available in the module cache.
type Postgres struct {
	conn   *pgx.Conn
	logger *slog.Logger
}

// NewPostgres creates a pgx connection and verifies connectivity.
func NewPostgres(ctx context.Context, dsn string, logger *slog.Logger) (*Postgres, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	logger.Info("postgres connected")

	return &Postgres{conn: conn, logger: logger}, nil
}

// Close releases the connection.
func (p *Postgres) Close() {
	_ = p.conn.Close(context.Background())
}

// SearchOffers performs semantic similarity search over scored_offer.
// If QueryEmbedding is empty, results are ordered by score DESC instead.
// All results are scoped to params.ProfileID (ADR-010).
func (p *Postgres) SearchOffers(ctx context.Context, params SearchParams) ([]*SearchResult, int, error) {
	conditions := []string{"profile_id = $1"}
	args := []any{params.ProfileID}
	idx := 2

	if len(params.Locations) > 0 {
		conditions = append(conditions, fmt.Sprintf("location = ANY($%d)", idx))
		args = append(args, params.Locations)
		idx++
	}
	if len(params.Companies) > 0 {
		conditions = append(conditions, fmt.Sprintf("company = ANY($%d)", idx))
		args = append(args, params.Companies)
		idx++
	}
	if len(params.Sources) > 0 {
		conditions = append(conditions, fmt.Sprintf("source = ANY($%d)", idx))
		args = append(args, params.Sources)
		idx++
	}
	if params.MinScore > 0 {
		conditions = append(conditions, fmt.Sprintf("score >= $%d", idx))
		args = append(args, params.MinScore)
		idx++
	}
	if params.DaysAgo > 0 {
		conditions = append(conditions, fmt.Sprintf("scored_at > NOW() - ($%d || ' days')::interval", idx))
		args = append(args, params.DaysAgo)
		idx++
	}
	if params.RemoteOnly {
		conditions = append(conditions, "remote = true")
	}
	if params.MinSalaryEUR > 0 {
		conditions = append(conditions, fmt.Sprintf("salary_min_eur >= $%d", idx))
		args = append(args, params.MinSalaryEUR)
		idx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count total matching offers for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM scored_offer %s", where)
	var total int
	if err := p.conn.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting offers: %w", err)
	}

	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var selectQuery string
	if len(params.QueryEmbedding) > 0 {
		conditions = append(conditions, "embedding IS NOT NULL")
		where = "WHERE " + strings.Join(conditions, " AND ")
		// Pass the embedding as a typed parameter using pgx text format.
		// $idx is appended here; other filters already occupy $1..$idx-1.
		args = append(args, embeddingToString(params.QueryEmbedding))
		embIdx := idx
		selectQuery = fmt.Sprintf(`
			SELECT
				offer_id, profile_id, title, company, location, source, url,
				score, reasoning, skill_matches, skill_gaps, reviewed, saved,
				scored_at, posted_at,
				(1 - (embedding <=> $%d::vector))::float4 AS similarity
			FROM scored_offer
			%s
			ORDER BY embedding <=> $%d::vector
			LIMIT %d OFFSET %d`,
			embIdx, where, embIdx, limit, params.Offset,
		)
	} else {
		selectQuery = fmt.Sprintf(`
			SELECT
				offer_id, profile_id, title, company, location, source, url,
				score, reasoning, skill_matches, skill_gaps, reviewed, saved,
				scored_at, posted_at,
				0.0::float4 AS similarity
			FROM scored_offer
			%s
			ORDER BY score DESC, scored_at DESC
			LIMIT %d OFFSET %d`,
			where, limit, params.Offset,
		)
	}

	rows, err := p.conn.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying scored offers: %w", err)
	}
	defer rows.Close()

	results, err := scanOfferRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// GetSimilarOffers returns offers similar to the given embedding, excluding offer_id.
// Scoped to profile_id (ADR-010). Single table scan, no JOIN (ADR-011).
func (p *Postgres) GetSimilarOffers(
	ctx context.Context,
	profileID, excludeOfferID string,
	embedding []float32,
	limit int32,
	daysAgo int32,
) ([]*SearchResult, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding must not be empty")
	}

	if limit <= 0 {
		limit = 5
	}

	conditions := []string{
		"profile_id = $1",
		"offer_id != $2",
		"embedding IS NOT NULL",
	}
	args := []any{profileID, excludeOfferID}
	idx := 3

	if daysAgo > 0 {
		conditions = append(conditions, fmt.Sprintf("scored_at > NOW() - ($%d || ' days')::interval", idx))
		args = append(args, daysAgo)
		idx++
	}

	// Append embedding as a typed parameter.
	args = append(args, embeddingToString(embedding))
	embIdx := idx

	where := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			offer_id, profile_id, title, company, location, source, url,
			score, reasoning, skill_matches, skill_gaps, reviewed, saved,
			scored_at, posted_at,
			(1 - (embedding <=> $%d::vector))::float4 AS similarity
		FROM scored_offer
		%s
		ORDER BY embedding <=> $%d::vector
		LIMIT %d`,
		embIdx, where, embIdx, limit,
	)

	rows, err := p.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying similar offers: %w", err)
	}
	defer rows.Close()

	return scanOfferRows(rows)
}

// StoreOffer upserts the embedding for an existing offer.
// The offer row must already exist (inserted by the fetcher service).
func (p *Postgres) StoreOffer(ctx context.Context, offerID string, embedding []float32) error {
	if len(embedding) == 0 {
		return fmt.Errorf("embedding must not be empty")
	}

	// Pass embedding as a typed parameter — no string interpolation in SQL.
	query := "UPDATE offer SET embedding = $1::vector WHERE id = $2"

	tag, err := p.conn.Exec(ctx, query, embeddingToString(embedding), offerID)
	if err != nil {
		return fmt.Errorf("updating offer embedding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOfferNotFound
	}

	return nil
}

// GetMarketContext returns the top scored offers matching a role/region/topic query.
// Uses text-based filtering + score ordering — no embedding required.
// Scoped to profile_id (ADR-010).
func (p *Postgres) GetMarketContext(
	ctx context.Context,
	profileID, role, region, topic string,
	daysAgo int32,
	maxOffers int32,
) ([]*SearchResult, int, error) {
	if maxOffers <= 0 {
		maxOffers = 10
	}

	conditions := []string{"profile_id = $1"}
	args := []any{profileID}
	idx := 2

	if role != "" {
		conditions = append(conditions, fmt.Sprintf("title ILIKE $%d", idx))
		args = append(args, "%"+role+"%")
		idx++
	}
	if region != "" {
		conditions = append(conditions, fmt.Sprintf("(location ILIKE $%d OR remote = true)", idx))
		args = append(args, "%"+region+"%")
		idx++
	}
	if topic != "" {
		conditions = append(conditions,
			fmt.Sprintf("(title ILIKE $%d OR reasoning ILIKE $%d)", idx, idx))
		args = append(args, "%"+topic+"%")
		idx++
	}
	if daysAgo > 0 {
		conditions = append(conditions, fmt.Sprintf("scored_at > NOW() - ($%d || ' days')::interval", idx))
		args = append(args, daysAgo)
		idx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM scored_offer %s", where)
	var total int
	if err := p.conn.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting market context offers: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT
			offer_id, profile_id, title, company, location, source, url,
			score, reasoning, skill_matches, skill_gaps, reviewed, saved,
			scored_at, posted_at, 0.0::float4 AS similarity
		FROM scored_offer
		%s
		ORDER BY score DESC, scored_at DESC
		LIMIT %d`,
		where, maxOffers,
	)

	rows, err := p.conn.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying market context: %w", err)
	}
	defer rows.Close()

	results, err := scanOfferRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// ErrOfferNotFound is returned when StoreOffer cannot find the target offer row.
var ErrOfferNotFound = fmt.Errorf("offer not found")

// --- helpers ---

// embeddingToString converts a float32 slice to the PostgreSQL vector literal format.
// Example: [0.1, 0.2, 0.3] → "[0.1,0.2,0.3]"
func embeddingToString(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// pgxRows is the subset of pgx.Rows used by scanOfferRows.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// scanOfferRows scans pgx rows into SearchResult slice.
func scanOfferRows(rows pgxRows) ([]*SearchResult, error) {
	var results []*SearchResult

	for rows.Next() {
		r := &SearchResult{}
		if err := rows.Scan(
			&r.OfferID,
			&r.ProfileID,
			&r.Title,
			&r.Company,
			&r.Location,
			&r.Source,
			&r.URL,
			&r.Score,
			&r.Reasoning,
			&r.SkillMatches,
			&r.SkillGaps,
			&r.Reviewed,
			&r.Saved,
			&r.ScoredAt,
			&r.PostedAt,
			&r.Similarity,
		); err != nil {
			return nil, fmt.Errorf("scanning offer row: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating offer rows: %w", err)
	}

	return results, nil
}
