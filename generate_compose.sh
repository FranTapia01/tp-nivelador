#!/usr/bin/env bash

# Validacion
if [ -z "$1" ] || ! [[ "$1" =~ ^[0-9]+$ ]] || [ "$1" -le 0 ]; then
    echo "Uso: $0 <cantidad_de_clientes>"
    exit 1
fi

CLIENTS_COUNT=$1
OUTPUT_FILE="docker-compose.yaml"

# Escribir la cabecera y el servicio del servidor
cat <<EOF > "$OUTPUT_FILE"
services:
  server:
    build:
      dockerfile: Dockerfile
    container_name: server
    environment:
      - PYTHONUNBUFFERED=1
      - SERVER_HOST=server
      - SERVER_PORT=5678

EOF

# Generar cada cliente en un bucle
for ((i=0; i<CLIENTS_COUNT; i++)); do
cat <<EOF >> "$OUTPUT_FILE"
  client_$i:
    build:
      context: ./services/client
      dockerfile: Dockerfile
    container_name: client_$i
    depends_on:
      - server
    environment:
      - AGENCY_ID=$i
      - SERVER_HOST=server
      - SERVER_PORT=5678

EOF
done

echo "Archivo $OUTPUT_FILE generado exitosamente con $CLIENTS_COUNT clientes."