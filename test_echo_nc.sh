#!/usr/bin/env bash

NETWORK_NAME=$(docker network ls --filter "name=_default" --format "{{.Name}}" | head -n 1)

if [ -z "$NETWORK_NAME" ]; then
    echo "Error: No se encontró la red interna de Docker Compose. ¿Está levantado el sistema?"
    exit 1
fi

echo "Conectando mediante contenedor efímero en la red: $NETWORK_NAME..."

# Correr un contenedor efímero (--rm) con netcat apuntando a 'server:5678'
docker run --rm --network "$NETWORK_NAME" busybox sh -c "echo 'Hello World desde contenedor' | nc server 5678"