#!/bin/bash
# ==============================================================================
# store_offer.sh — Store an offer embedding in pgvector
#
# Usage:
#   ./store_offer.sh
#   ./store_offer.sh <profile_id> <offer_id>
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/rag-service 50052:50052 -n jobradar
# ==============================================================================

set -e

HOST="${RAG_HOST:-localhost:50052}"
PROFILE_ID="${1:-a1b2c3d4-e5f6-7890-abcd-ef1234567890}"
OFFER_ID="${2:-f47ac10b-58cc-4372-a567-0e02b2c3d479}"

# Generate a fake 1024-dim embedding (all 0.01 for demo purposes)
EMBEDDING=$(python3 -c "import json; print(json.dumps([round(0.01 * (i % 100), 4) for i in range(1024)]))")

echo "→ Storing offer embedding..."
echo "  Host:       $HOST"
echo "  Profile ID: $PROFILE_ID"
echo "  Offer ID:   $OFFER_ID"
echo "  Embedding:  [1024 dimensions]"
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d "{
    \"profile_id\": \"$PROFILE_ID\",
    \"offer_id\":   \"$OFFER_ID\",
    \"embedding\":  $EMBEDDING,
    \"metadata\": {
      \"title\":    \"Staff Backend Engineer\",
      \"company\":  \"Factorial HR\",
      \"location\": \"Barcelona, Spain\",
      \"source\":   \"linkedin\",
      \"url\":      \"https://www.linkedin.com/jobs/view/3987654321\",
      \"raw_text\": \"We are looking for a Staff Backend Engineer with strong Go and Kubernetes experience.\",
      \"posted_at\": 1742601600
    }
  }" \
  "$HOST" \
  rag.v1.RAGService/StoreOffer)

STORED=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('stored',''))")
RESP_OFFER_ID=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('offerId',''))")

echo "✓ Offer stored"
echo "  Offer ID: $RESP_OFFER_ID"
echo "  Stored:   $STORED"
