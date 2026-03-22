#!/bin/bash
# ==============================================================================
# embed_text.sh — Generate an embedding for a single text
#
# Usage:
#   ./embed_text.sh "Your text here"
#   ./embed_text.sh "Staff Backend Engineer Go Kubernetes" document
#   ./embed_text.sh "Go Kubernetes jobs in Barcelona" query
#
# Purpose:
#   document — use when storing text (offer, CV)
#   query    — use when searching against stored documents
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/embedder 50051:50051 -n jobradar
# ==============================================================================

set -e

HOST="${EMBEDDER_HOST:-localhost:50051}"
TEXT="${1:-Staff Backend Engineer Go Kubernetes}"
PURPOSE="${2:-document}"

# Map purpose string to proto enum
case "$PURPOSE" in
  document) PURPOSE_ENUM="EMBED_PURPOSE_DOCUMENT" ;;
  query)    PURPOSE_ENUM="EMBED_PURPOSE_QUERY" ;;
  *)        echo "Unknown purpose: $PURPOSE (use 'document' or 'query')"; exit 1 ;;
esac

echo "→ Generating embedding..."
echo "  Host:    $HOST"
echo "  Text:    $TEXT"
echo "  Purpose: $PURPOSE"
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d "{\"text\": \"$TEXT\", \"purpose\": \"$PURPOSE_ENUM\"}" \
  "$HOST" \
  embedder.v1.EmbedderService/EmbedText)

# Extract key fields
MODEL=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('model',''))")
TOKENS=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tokens',''))")
LATENCY=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('latencyMs',''))")
DIMS=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('embedding',[])))")
PREVIEW=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); e=d.get('embedding',[]); print([round(v,4) for v in e[:5]])")

echo "✓ Embedding generated"
echo "  Model:      $MODEL"
echo "  Dimensions: $DIMS"
echo "  Tokens:     $TOKENS"
echo "  Latency:    ${LATENCY}ms"
echo "  Preview:    $PREVIEW ..."
