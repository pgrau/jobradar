package db

import (
	"testing"
)

func TestEmbeddingToString(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
		want  string
	}{
		{
			name:  "single value",
			input: []float32{0.5},
			want:  "[0.5]",
		},
		{
			name:  "multiple values",
			input: []float32{0.1, 0.2, 0.3},
			want:  "[0.1,0.2,0.3]",
		},
		{
			name:  "zero values",
			input: []float32{0.0, 0.0},
			want:  "[0,0]",
		},
		{
			name:  "negative values",
			input: []float32{-0.5, 0.5},
			want:  "[-0.5,0.5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := embeddingToString(tt.input)
			if got != tt.want {
				t.Errorf("embeddingToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStoreOffer_EmptyEmbedding_ReturnsError(t *testing.T) {
	p := &Postgres{} // no real connection needed for this validation
	err := p.StoreOffer(t.Context(), "some-id", nil)
	if err == nil {
		t.Fatal("expected error for empty embedding, got nil")
	}
}

func TestGetSimilarOffers_EmptyEmbedding_ReturnsError(t *testing.T) {
	p := &Postgres{}
	_, err := p.GetSimilarOffers(t.Context(), "profile-id", "offer-id", nil, 5, 30)
	if err == nil {
		t.Fatal("expected error for empty embedding, got nil")
	}
}
