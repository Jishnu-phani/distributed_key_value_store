#!/bin/bash
set -e

BASE="http://localhost:8080/kv"
PASS=0
FAIL=0

check() {
  local desc="$1"
  local expected="$2"
  local actual="$3"
  if [ "$expected" == "$actual" ]; then
    echo "✅ PASS: $desc"
    PASS=$((PASS+1))
  else
    echo "❌ FAIL: $desc"
    echo "   expected: $expected"
    echo "   actual:   $actual"
    FAIL=$((FAIL+1))
  fi
}

echo "--- Test 1: GET missing key returns 404 ---"
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/missing")
check "404 for missing key" "404" "$status"

echo "--- Test 2: PUT then GET returns correct value ---"
curl -s -X PUT "$BASE/foo" -d '{"value": "bar"}' > /dev/null
resp=$(curl -s "$BASE/foo")
value=$(echo "$resp" | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
check "value round-trips correctly" "bar" "$value"

echo "--- Test 3: PUT overwrites existing value ---"
curl -s -X PUT "$BASE/foo" -d '{"value": "baz"}' > /dev/null
resp=$(curl -s "$BASE/foo")
value=$(echo "$resp" | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
check "overwrite works" "baz" "$value"

echo "--- Test 4: DELETE removes the key ---"
curl -s -X DELETE "$BASE/foo" > /dev/null
status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/foo")
check "404 after delete" "404" "$status"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1