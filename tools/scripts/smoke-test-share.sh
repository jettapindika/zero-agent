#!/bin/bash
# Smoke test for the share/collab feature.
# Assumes the daemon is running on localhost:8910 with auth enabled.
# Usage: ./tools/scripts/smoke-test-share.sh <session_token>

set -e

API_BASE="http://127.0.0.1:8910"
TOKEN="${1:-}"

if [ -z "$TOKEN" ]; then
  echo "Usage: $0 <session_token>"
  echo "  Get token from ~/.zero/auth.json or browser devtools after sign-in"
  exit 1
fi

AUTH_HEADER="Authorization: Bearer $TOKEN"
CLIENT_ID_HEADER="X-Zero-Client-ID: smoke-test-$(date +%s)"

echo "=== Step 1: Verify /auth/me returns sessionToken ==="
AUTH_ME=$(curl -sS -H "$AUTH_HEADER" "$API_BASE/auth/me")
echo "$AUTH_ME" | jq -e '.sessionToken' >/dev/null || {
  echo "ERROR: /auth/me did not return sessionToken field"
  exit 1
}
echo "✓ /auth/me returns sessionToken"

echo ""
echo "=== Step 2: Get client identity ==="
IDENTITY=$(curl -sS -H "$AUTH_HEADER" -H "$CLIENT_ID_HEADER" "$API_BASE/identity")
echo "$IDENTITY"
CLIENT_ID=$(echo "$IDENTITY" | jq -r '.clientId')
if [ "$CLIENT_ID" = "null" ] || [ -z "$CLIENT_ID" ]; then
  echo "ERROR: Could not get clientId from /identity"
  exit 1
fi
echo "✓ Got clientId: $CLIENT_ID"

echo ""
echo "=== Step 3: Create collab room ==="
ROOM_RESULT=$(curl -sS -X POST \
  -H "$AUTH_HEADER" \
  -H "Content-Type: application/json" \
  -H "X-Zero-Client-ID: $CLIENT_ID" \
  -d '{"name":"smoke-test-room","projectId":"test","permissions":{"read":true,"write":false,"execute":false},"maxGuests":5}' \
  "$API_BASE/collab/rooms")
echo "$ROOM_RESULT"
ROOM_ID=$(echo "$ROOM_RESULT" | jq -r '.id')
INVITE_TOKEN=$(echo "$ROOM_RESULT" | jq -r '.inviteToken')
if [ "$ROOM_ID" = "null" ] || [ -z "$ROOM_ID" ]; then
  echo "ERROR: Could not create room"
  exit 1
fi
echo "✓ Created room: $ROOM_ID"
echo "✓ Invite token: ${INVITE_TOKEN:0:8}..."

echo ""
echo "=== Step 4: Get room details ==="
ROOM_DETAILS=$(curl -sS -H "$AUTH_HEADER" "$API_BASE/collab/rooms/$ROOM_ID")
echo "$ROOM_DETAILS" | jq '.'
echo "✓ Room details retrieved"

echo ""
echo "=== Step 5: Revoke room (cleanup) ==="
curl -sS -X POST -H "$AUTH_HEADER" -H "X-Zero-Client-ID: $CLIENT_ID" "$API_BASE/collab/rooms/$ROOM_ID/revoke"
echo "✓ Room revoked"

echo ""
echo "=== All smoke tests passed ==="
