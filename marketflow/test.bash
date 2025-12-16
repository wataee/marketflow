#!/bin/bash

set -e

ARCH=$(uname -m)
echo "Detected architecture: $ARCH"

if [ "$ARCH" != "x86_64" ]; then
    echo "This script is intended for AMD64/x86_64 architecture."
    exit 1
fi

# Загружаем Docker образы для AMD64
echo "Loading Docker images for AMD64..."
docker load -i exchange1_amd64.tar
docker load -i exchange2_amd64.tar
docker load -i exchange3_amd64.tar

# Устанавливаем имена образов, которые были загружены
IMAGE1="exchange1:latest"
IMAGE2="exchange2:latest"
IMAGE3="exchange3:latest"

# Удаляем старые контейнеры, если есть
for NAME in exchange1 exchange2 exchange3; do
    if [ "$(docker ps -aq -f name=$NAME)" ]; then
        echo "Removing existing container: $NAME"
        docker rm -f $NAME
    fi
done

# Запускаем контейнеры с загруженными образами
echo "Starting containers..."
docker run -p 40101:40101 --name exchange1 -d $IMAGE1
docker run -p 40102:40102 --name exchange2 -d $IMAGE2
docker run -p 40103:40103 --name exchange3 -d $IMAGE3

echo "All containers started successfully!"
