#!/bin/bash
# ==============================================================================
# embed_batch.sh — Generate embeddings for multiple texts in a single call
#
# Usage:
#   ./embed_batch.sh
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/embedder 50051:50051 -n jobradar
# ==============================================================================

set -e

HOST="${EMBEDDER_HOST:-localhost:50051}"

echo "→ Generating batch embeddings..."
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d '{
    "items": [
      {"id": "1", "text": "Staff Backend Engineer Go Kubernetes", "purpose": "EMBED_PURPOSE_DOCUMENT"},
      {"id": "2", "text": "Senior PHP Developer Symfony DDD", "purpose": "EMBED_PURPOSE_DOCUMENT"},
      {"id": "3", "text": "AI Platform Engineer Python LLMs", "purpose": "EMBED_PURPOSE_DOCUMENT"}
    ]
  }' \
  "$HOST" \
  embedder.v1.EmbedderService/EmbedBatch)

MODEL=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('model',''))")
COUNT=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('results',[])))")

echo "✓ Batch embeddings generated"
echo "  Model:   $MODEL"
echo "  Count:   $COUNT embeddings"
echo ""

echo "$RESPONSE" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d.get('results', []):
    dims = len(r.get('embedding', []))
    cached = r.get('cached', False)
    preview = [round(v, 4) for v in r['embedding'][:3]]
    print(f\"  [{r['id']}] dims={dims} cached={cached} preview={preview}...\")
"
