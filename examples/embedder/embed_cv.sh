#!/bin/bash
# ==============================================================================
# embed_cv.sh — Generate an embedding for a user CV
#
# Usage:
#   ./embed_cv.sh <profile_id> <cv_text>
#   ./embed_cv.sh "user-123" "10 years experience in Go, Kubernetes, distributed systems..."
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/embedder 50051:50051 -n jobradar
# ==============================================================================

set -e

HOST="${EMBEDDER_HOST:-localhost:50051}"
PROFILE_ID="${1:-test-profile-001}"
CV_TEXT="${2:-Tech Lead with 20 years experience. Expert in Go, PHP, Kubernetes, DDD, distributed systems, event-driven architecture. Built high-traffic APIs processing 5M requests/week.}"

echo "→ Generating CV embedding..."
echo "  Host:       $HOST"
echo "  Profile ID: $PROFILE_ID"
echo "  CV length:  ${#CV_TEXT} chars"
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d "{\"profile_id\": \"$PROFILE_ID\", \"cv_text\": \"$CV_TEXT\"}" \
  "$HOST" \
  embedder.v1.EmbedderService/EmbedCV)

MODEL=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('model',''))")
TOKENS=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tokens',''))")
DIMS=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('embedding',[])))")
PREVIEW=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); e=d.get('embedding',[]); print([round(v,4) for v in e[:5]])")

echo "✓ CV embedding generated"
echo "  Profile ID:  $PROFILE_ID"
echo "  Model:       $MODEL"
echo "  Dimensions:  $DIMS"
echo "  Tokens:      $TOKENS"
echo "  Preview:     $PREVIEW ..."
