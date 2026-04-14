#!/bin/bash

set -e

DN_COUNT_DEFAULT=2

if [ -n "${DN_COUNT:-}" ]; then
  :
elif [ -t 0 ]; then
  read -r -p "How many DataNodes do you want? [${DN_COUNT_DEFAULT}]: " DN_COUNT
else
  DN_COUNT="${DN_COUNT_DEFAULT}"
fi

DN_COUNT="${DN_COUNT:-${DN_COUNT_DEFAULT}}"

if ! [[ "${DN_COUNT}" =~ ^[0-9]+$ ]] || [ "${DN_COUNT}" -lt 1 ]; then
  echo "ERROR: DataNode count must be an integer >= 1"
  exit 1
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  echo "ERROR: docker compose is required (or docker-compose)."
  exit 1
fi

# ensure host directories exist
mkdir -p "$(pwd)/data/nn"
mkdir -p "$(pwd)/history"
touch "$(pwd)/history/.bash_history"

# cleanup legacy containers from previous non-compose runs
docker rm -f hadoop-namenode hadoop-dev >/dev/null 2>&1 || true
docker ps -a --format '{{.Names}}' | grep '^hadoop-datanode-' | xargs -r docker rm -f >/dev/null 2>&1 || true

# recreate cluster with requested DataNode count
${COMPOSE_CMD} down --remove-orphans >/dev/null 2>&1 || true
${COMPOSE_CMD} up -d --scale datanode="${DN_COUNT}"

echo "Waiting for NameNode to be ready..."
for i in $(seq 1 30); do
  if ${COMPOSE_CMD} exec -T namenode /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "Cluster started: 1 NameNode + ${DN_COUNT} DataNode(s)"
echo "- NameNode UI:  http://localhost:9870"
echo "- YARN UI:      http://localhost:8088"
echo "- JobHistory:   http://localhost:19888"

# open interactive shell in NameNode service
if [ "${NO_ATTACH:-0}" = "1" ]; then
  echo "NO_ATTACH=1 set, skipping interactive shell attach."
else
  ${COMPOSE_CMD} exec namenode bash -lc '
    export HADOOP_HOME=/opt/hadoop-3.2.1
    export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
    exec env HADOOP_HOME="${HADOOP_HOME}" PATH="${PATH}" bash -i
  '
fi