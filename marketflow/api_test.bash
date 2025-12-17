#!/bin/bash

BASE_URL="http://localhost:8080"

echo "=== Testing MarketFlow API ==="
echo

echo "1. Health Check..."
curl -s "$BASE_URL/health" | jq
echo

echo "2. Waiting 10 seconds for data to accumulate..."
sleep 10

echo "3. Testing Latest Prices..."
echo "   - Latest BTCUSDT (any exchange):"
curl -s "$BASE_URL/prices/latest/BTCUSDT" | jq
echo "   - Latest ETHUSDT from exchange1:"
curl -s "$BASE_URL/prices/latest/exchange1/ETHUSDT" | jq
echo

echo "4. Testing Highest Prices..."
echo "   - Highest BTCUSDT (last minute):"
curl -s "$BASE_URL/prices/highest/BTCUSDT" | jq
echo "   - Highest SOLUSDT (last 30s):"
curl -s "$BASE_URL/prices/highest/SOLUSDT?period=30s" | jq
echo

echo "5. Testing Lowest Prices..."
echo "   - Lowest DOGEUSDT (last minute):"
curl -s "$BASE_URL/prices/lowest/DOGEUSDT" | jq
echo "   - Lowest TONUSDT from exchange2 (last 15s):"
curl -s "$BASE_URL/prices/lowest/exchange2/TONUSDT?period=15s" | jq
echo

echo "6. Testing Average Prices..."
echo "   - Average ETHUSDT (last minute):"
curl -s "$BASE_URL/prices/average/ETHUSDT" | jq
echo "   - Average BTCUSDT from exchange3 (last 20s):"
curl -s "$BASE_URL/prices/average/exchange3/BTCUSDT?period=20s" | jq
echo

echo "7. Testing Mode Switching..."
echo "   - Switch to Test Mode:"
curl -X POST "$BASE_URL/mode/test"
echo
sleep 2
echo "   - Switch back to Live Mode:"
curl -X POST "$BASE_URL/mode/live"
echo
echo

echo "=== All tests completed ==="