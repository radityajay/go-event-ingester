#!/usr/bin/env bash
# Load test using vegeta (https://github.com/tsenart/vegeta)
# Install: brew install vegeta
#
# Usage:
#   ./loadtest/vegeta.sh [rate] [duration]
#   ./loadtest/vegeta.sh 5000 30s

set -euo pipefail

RATE=${1:-1000}
DURATION=${2:-30s}
BASE_URL=${BASE_URL:-http://localhost:8080}

echo "=== Event Ingester Load Test ==="
echo "Target: ${BASE_URL}"
echo "Rate:   ${RATE} req/s"
echo "Duration: ${DURATION}"
echo ""

# Generate attack body.
generate_body() {
    cat <<EOF
{
  "user_id": "usr_$(( RANDOM % 100000 ))",
  "event_type": "page_view",
  "page": "/home",
  "device": "mobile",
  "country": "ID"
}
EOF
}

BODY_FILE=$(mktemp)
generate_body > "$BODY_FILE"

# Run attack.
echo "POST ${BASE_URL}/api/v1/events" | \
    vegeta attack \
        -rate="${RATE}" \
        -duration="${DURATION}" \
        -body="$BODY_FILE" \
        -header="Content-Type: application/json" | \
    vegeta report

# Cleanup.
rm -f "$BODY_FILE"

echo ""
echo "=== Check system metrics ==="
curl -s "${BASE_URL}/metrics" | python3 -m json.tool 2>/dev/null || curl -s "${BASE_URL}/metrics"
