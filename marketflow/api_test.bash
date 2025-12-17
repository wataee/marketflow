#!/bin/bash

# Цвета для вывода
GREEN='\033[0-9;32m'
RED='\033[0-9;31m'
NC='\033[0m' # No Color

BASE_URL="http://localhost:8080"

echo -e "${GREEN}=== MarketFlow Full Integration Test ===${NC}"

# --- 1. Проверка компиляции ---
echo -e "\n[1/6] Checking Compilation..."
go build -o marketflow ./cmd/marketflow
if [ $? -eq 0 ]; then
    echo -e "${GREEN}PASS: Compiled successfully${NC}"
else
    echo -e "${RED}FAIL: Compilation failed${NC}"
    exit 1
fi

# --- 2. Проверка Usage/Help ---
echo -e "\n[2/6] Checking Usage Info..."
./marketflow --help | grep -q "Usage:"
if [ $? -eq 0 ]; then
    echo -e "${GREEN}PASS: Usage info printed${NC}"
else
    echo -e "${RED}FAIL: --help not working correctly${NC}"
fi

# Проверка, запущен ли сервер для дальнейших тестов
if ! curl -s "$BASE_URL/health" > /dev/null; then
    echo -e "${RED}Error: Server is not running at $BASE_URL. Please start it first.${NC}"
    exit 1
fi

# --- 3. Health Check ---
echo -e "\n[3/6] Checking Health API..."
HEALTH=$(curl -s "$BASE_URL/health")
echo "Response: $HEALTH"
if [[ $HEALTH == *"ok"* ]]; then
    echo -e "${GREEN}PASS: Systems are healthy${NC}"
else
    echo -e "${RED}FAIL: Health check degraded${NC}"
fi

# --- 4. Тестирование API данных ---
echo -e "\n[4/6] Testing Data API (Latest & Aggregated)..."
SYMBOLS=("BTCUSDT" "ETHUSDT")

for s in "${SYMBOLS[@]}"; do
    echo -n "Testing latest $s: "
    curl -s "$BASE_URL/prices/latest/$s" | grep -q "price" && echo -e "${GREEN}OK${NC}" || echo -e "${RED}FAIL${NC}"
done

echo -n "Testing statistics (Average BTCUSDT 1m): "
curl -s "$BASE_URL/prices/average/BTCUSDT?period=1m" | grep -q "average" && echo -e "${GREEN}OK${NC}" || echo -e "${RED}FAIL${NC}"

# --- 5. Переключение режимов (Hexagonal Architecture test) ---
echo -e "\n[5/6] Testing Mode Switching..."

echo -n "Switching to TEST mode: "
curl -s -X POST "$BASE_URL/mode/test" | grep -q "test" && echo -e "${GREEN}SUCCESS${NC}" || echo -e "${RED}FAIL${NC}"

sleep 2
echo -n "Verifying test data: "
curl -s "$BASE_URL/prices/latest/BTCUSDT" | grep -q "BTCUSDT" && echo -e "${GREEN}OK${NC}" || echo -e "${RED}FAIL${NC}"

echo -n "Switching back to LIVE mode: "
curl -s -X POST "$BASE_URL/mode/live" | grep -q "live" && echo -e "${GREEN}SUCCESS${NC}" || echo -e "${RED}FAIL${NC}"

# --- 6. Graceful Shutdown ---
echo -e "\n[6/6] Testing Graceful Shutdown (SIGTERM)..."

# Ищем PID более надежным способом
APP_PID=$(pgrep -f "./marketflow" | head -n 1)

if [ -z "$APP_PID" ]; then
    echo -e "${RED}FAIL: Could not find marketflow process ID${NC}"
else
    echo "Sending SIGTERM to PID $APP_PID..."
    # Пробуем убить процесс. Если обычный kill не сработал, пишем ошибку понятнее
    kill -TERM "$APP_PID" 2>/dev/null || sudo kill -TERM "$APP_PID" 2>/dev/null
    
    SUCCESS=false
    for i in {1..10}; do
        if ! ps -p "$APP_PID" > /dev/null; then
            SUCCESS=true
            break
        fi
        sleep 1
    done

    if [ "$SUCCESS" = true ]; then
        echo -e "${GREEN}PASS: Application exited cleanly${NC}"
    else
        echo -e "${RED}FAIL: Application did not stop. Check if you have permissions or if SIGTERM is handled.${NC}"
    fi
fi

echo -e "\n${GREEN}=== All tests completed ===${NC}"