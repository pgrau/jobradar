#!/bin/bash
# ==============================================================================
# get_similar_offers.sh — Find offers similar to a reference offer
#
# Usage:
#   ./get_similar_offers.sh
#   ./get_similar_offers.sh <profile_id> <offer_id>
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/rag-service 50052:50052 -n jobradar
# ==============================================================================

set -e

HOST="${RAG_HOST:-localhost:50052}"
PROFILE_ID="${1:-a1b2c3d4-e5f6-7890-abcd-ef1234567890}"
OFFER_ID="${2:-f47ac10b-58cc-4372-a567-0e02b2c3d479}"
LIMIT="${3:-5}"
DAYS_AGO="${4:-30}"

# Generate a fake 1024-dim embedding (all 0.01 for demo purposes)
EMBEDDING=$(python3 -c "import json; print(json.dumps([round(0.01 * (i % 100), 4) for i in range(1024)]))")

echo "→ Finding similar offers..."
echo "  Host:       $HOST"
echo "  Profile ID: $PROFILE_ID"
echo "  Offer ID:   $OFFER_ID"
echo "  Limit:      $LIMIT"
echo "  Days ago:   $DAYS_AGO"
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d "{
    \"profile_id\": \"$PROFILE_ID\",
    \"offer_id\":   \"$OFFER_ID\",
    \"embedding\":  $EMBEDDING,
    \"limit\":      $LIMIT,
    \"days_ago\":   $DAYS_AGO
  }" \
  "$HOST" \
  rag.v1.RAGService/GetSimilarOffers)

COUNT=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('results',[])))")

echo "✓ Similar offers found"
echo "  Count: $COUNT"
echo ""

echo "$RESPONSE" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d.get('results', []):
    print(f\"  [{r.get('score',0):3.0f}] {r.get('title','')}\")
    print(f\"        {r.get('company','')} — {r.get('location','')}\")
    print(f\"        similarity={r.get('similarity',0):.3f}  source={r.get('source','')}\")
    print(f\"        reasoning: {r.get('reasoning','')}\")
    print()
"
