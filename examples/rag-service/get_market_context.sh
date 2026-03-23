#!/bin/bash
# ==============================================================================
# get_market_context.sh — Retrieve RAG-augmented market context for a role/topic
#
# Usage:
#   ./get_market_context.sh
#   ./get_market_context.sh <profile_id> <role> <topic>
#   ./get_market_context.sh "a1b2c3d4-..." "Staff Backend Engineer" "Go"
#
# Requirements:
#   - grpcurl (brew install grpcurl)
#   - kubectl port-forward svc/rag-service 50052:50052 -n jobradar
# ==============================================================================

set -e

HOST="${RAG_HOST:-localhost:50052}"
PROFILE_ID="${1:-a1b2c3d4-e5f6-7890-abcd-ef1234567890}"
ROLE="${2:-Staff Backend Engineer}"
TOPIC="${3:-Go}"
REGION="${4:-Europe}"
DAYS_AGO="${5:-30}"
MAX_OFFERS="${6:-10}"

echo "→ Fetching market context..."
echo "  Host:       $HOST"
echo "  Profile ID: $PROFILE_ID"
echo "  Role:       $ROLE"
echo "  Topic:      $TOPIC"
echo "  Region:     $REGION"
echo "  Days ago:   $DAYS_AGO"
echo ""

RESPONSE=$(grpcurl -plaintext \
  -d "{
    \"profile_id\": \"$PROFILE_ID\",
    \"role\":       \"$ROLE\",
    \"region\":     \"$REGION\",
    \"topic\":      \"$TOPIC\",
    \"days_ago\":   $DAYS_AGO,
    \"max_offers\": $MAX_OFFERS
  }" \
  "$HOST" \
  rag.v1.RAGService/GetMarketContext)

TOTAL=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('totalOffers',0))")
PERIOD=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('period',''))")
COUNT=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('contextOffers',[])))")

echo "✓ Market context retrieved"
echo "  Period:        $PERIOD"
echo "  Total offers:  $TOTAL"
echo "  Context size:  $COUNT offers"
echo ""

echo "$RESPONSE" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d.get('contextOffers', []):
    print(f\"  [{r.get('score',0):3.0f}] {r.get('title','')}\")
    print(f\"        {r.get('company','')} — {r.get('location','')}\")
    print(f\"        similarity={r.get('similarity',0):.3f}  source={r.get('source','')}\")
    skills = r.get('skillMatches', [])
    print(f\"        skills={skills}\")
    print()
"
