#!/bin/bash

# URL сервиса marketflow
BASE_URL="http://localhost:8080"

# Список торговых пар (подставьте те, что у вас в config.json)
TRADING_PAIRS=("BTCUSD" "ETHUSD" "SOLUSD" "XRPUSD")

echo "=== Проверка работы marketflow с эмуляторами ==="

for pair in "${TRADING_PAIRS[@]}"; do
    echo
    echo "Торговая пара: $pair"
    response=$(curl -s "${BASE_URL}/prices/latest/${pair}")
    if [ $? -eq 0 ] && [ -n "$response" ]; then
        echo "Последняя цена: $response"
    else
        echo "Ошибка: не удалось получить цену для $pair"
    fi
done

echo
echo "=== Проверка завершена ==="
