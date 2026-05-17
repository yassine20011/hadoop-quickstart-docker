#!/bin/bash

set -e

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  echo "ERROR: docker compose is required."; exit 1
fi

DATANODES_COMPOSE="$(pwd)/docker-compose.datanodes.yml"
COMPOSE_FILES="-f docker-compose.yml"
[ -f "${DATANODES_COMPOSE}" ] && COMPOSE_FILES+=" -f ${DATANODES_COMPOSE}"

${COMPOSE_CMD} ${COMPOSE_FILES} down --remove-orphans
echo "Cluster stopped."
