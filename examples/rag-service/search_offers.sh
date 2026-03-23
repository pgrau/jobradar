#!/bin/bash
# ==============================================================================
# search_offers.sh — Semantic search over ingested offers for a profile
#
# Usage:
#   ./search_offers.sh
#   ./search_offers.sh <profile_id> <query>
#   ./search_offers.sh "a1b2c3d4-..." "Staff Backend Engineer Go Kubernetes remote"
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/rag-service 50052:50052 -n jobradar
# ==============================================================================

set -e

HOST="${RAG_HOST:-localhost:50052}"
PROFILE_ID="${1:-a1b2c3d4-e5f6-7890-abcd-ef1234567890}"
QUERY="${2:-Staff Backend Engineer Go Kubernetes remote}"
LIMIT="${3:-5}"

echo "→ Searching offers..."
echo "  Host:       $HOST"
echo "  Profile ID: $PROFILE_ID"
echo "  Query:      $QUERY"
echo "  Limit:      $LIMIT"
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d "{
    \"profile_id\": \"$PROFILE_ID\",
    \"query\":      \"$QUERY\",
    \"limit\":      $LIMIT,
    \"offset\":     0
  }" \
  "$HOST" \
  rag.v1.RAGService/SearchOffers)

TOTAL=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total',0))")
COUNT=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('results',[])))")

echo "✓ Search complete"
echo "  Total:   $TOTAL offers found"
echo "  Results: $COUNT returned"
echo ""

echo "$RESPONSE" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d.get('results', []):
    print(f\"  [{r.get('score',0):3.0f}] {r.get('title','')}\")
    print(f\"        {r.get('company','')} — {r.get('location','')}\")
    print(f\"        similarity={r.get('similarity',0):.3f}  source={r.get('source','')}\")
    skills = r.get('skillMatches', [])
    gaps   = r.get('skillGaps', [])
    print(f\"        matches={skills}  gaps={gaps}\")
    print()
"
