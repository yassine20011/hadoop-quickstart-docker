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

if [ -n "${START_HIVE:-}" ]; then
  :
elif [ -t 0 ]; then
  read -r -p "Start Hive (metastore + HiveServer2)? [y/N]: " START_HIVE
else
  START_HIVE="n"
fi

case "${START_HIVE,,}" in
  y|yes) START_HIVE=1 ;;
  *)     START_HIVE=0 ;;
esac

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  echo "ERROR: docker compose is required (or docker-compose)."
  exit 1
fi

is_service_running() {
  local service="$1"
  ${COMPOSE_CMD} ${COMPOSE_FILES} ps --status running --services | grep -qx "${service}"
}

reset_hive_postgres_data() {
  local hive_data_dir
  hive_data_dir="$(pwd)/data/hive/postgresql"

  echo "Resetting Hive metastore PostgreSQL data..."
  mkdir -p "${hive_data_dir}"
  docker run --rm \
    -v "${hive_data_dir}:/var/lib/postgresql/data" \
    alpine:3.20 \
    sh -c 'rm -rf /var/lib/postgresql/data/* /var/lib/postgresql/data/.[!.]* /var/lib/postgresql/data/..?* || true'
}

attempt_hive_recovery_if_needed() {
  local recovered=0

  for i in $(seq 1 45); do
    if is_service_running hive-metastore && is_service_running hive-server; then
      return 0
    fi
    sleep 2
  done

  if ${COMPOSE_CMD} logs --no-color hive-metastore 2>/dev/null | grep -q "Version information not found in metastore"; then
    echo "WARN: Hive metastore schema state is invalid. Attempting automatic recovery..."
    recovered=1
    ${COMPOSE_CMD} ${COMPOSE_FILES} down --remove-orphans >/dev/null 2>&1 || true
    reset_hive_postgres_data
    ${COMPOSE_CMD} ${COMPOSE_FILES} up -d

    for i in $(seq 1 45); do
      if is_service_running hive-metastore && is_service_running hive-server; then
        echo "Hive recovery succeeded."
        return 0
      fi
      sleep 2
    done
  fi

  if [ "${recovered}" -eq 1 ]; then
    echo "ERROR: Hive recovery was attempted but Hive services are still not running."
  else
    echo "ERROR: Hive services failed to start."
  fi

  echo "Recent hive-metastore logs:"
  ${COMPOSE_CMD} logs --tail 120 hive-metastore || true
  echo "Recent hive-server logs:"
  ${COMPOSE_CMD} logs --tail 120 hive-server || true
  return 1
}

wait_for_port() {
  local service="$1" host="$2" port="$3" retries="${4:-60}"
  echo "Waiting for ${service} on ${host}:${port}..."
  for i in $(seq 1 "${retries}"); do
    if ${COMPOSE_CMD} exec -T "${service}" bash -c \
         "echo > /dev/tcp/${host}/${port}" 2>/dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "ERROR: ${service} port ${port} never opened."
  return 1
}

wait_for_hiveserver2_ready() {
  echo "Waiting for HiveServer2 to accept connections on port 10000..."
  wait_for_port hive-server localhost 10000 120
  for i in $(seq 1 120); do
    if ${COMPOSE_CMD} exec -T hive-server bash -lc "/opt/hive/bin/beeline -u jdbc:hive2://localhost:10000 -n hive -e '!quit' >/dev/null 2>&1" 2>/dev/null; then
      return 0
    fi
    sleep 2
  done

  echo "ERROR: HiveServer2 did not become ready in time."
  echo "Recent hive-server logs:"
  ${COMPOSE_CMD} logs --tail 120 hive-server || true
  return 1
}

# generate per-DataNode compose override with isolated volumes
DATANODES_COMPOSE="$(pwd)/docker-compose.datanodes.yml"
{
  echo "services:"
  for n in $(seq 1 "${DN_COUNT}"); do
    mkdir -p "$(pwd)/data/dn${n}"
    chmod 777 "$(pwd)/data/dn${n}"
    echo "  datanode${n}:"
    echo "    image: bde2020/hadoop-base:2.0.0-hadoop3.2.1-java8"
    echo "    container_name: hadoop-datanode-${n}"
    echo "    hostname: datanode${n}"
    echo "    depends_on:"
    echo "      - namenode"
    echo "    env_file:"
    echo "      - ./hadoop.env"
    echo "    environment:"
    echo "      CORE_CONF_fs_defaultFS: hdfs://namenode:8020"
    echo "    volumes:"
    echo "      - ./shared:/shared"
    echo "      - ./data/dn${n}:/tmp/hadoop-root"
    echo "    command:"
    echo "      - bash"
    echo "      - /shared/compose-datanode.sh"
  done
} > "${DATANODES_COMPOSE}"

COMPOSE_FILES="-f docker-compose.yml -f ${DATANODES_COMPOSE}"
mkdir -p "$(pwd)/data/hive/postgresql"
mkdir -p "$(pwd)/history"
touch "$(pwd)/history/.bash_history"
if ! chmod 777 "$(pwd)/data/hive/postgresql" 2>/dev/null; then
  echo "WARN: could not chmod data/hive/postgresql (likely owned by a container UID); continuing."
fi

# cleanup legacy containers from previous non-compose runs
docker rm -f hadoop-namenode hadoop-dev >/dev/null 2>&1 || true
docker ps -a --format '{{.Names}}' | grep '^hadoop-datanode-' | xargs -r docker rm -f >/dev/null 2>&1 || true

# recreate cluster with requested DataNode count
${COMPOSE_CMD} ${COMPOSE_FILES} down --remove-orphans >/dev/null 2>&1 || true
if [ "${START_HIVE}" = "1" ]; then
  ${COMPOSE_CMD} ${COMPOSE_FILES} up -d
else
  ${COMPOSE_CMD} ${COMPOSE_FILES} up -d \
    --scale postgres=0 \
    --scale hive-metastore=0 \
    --scale hive-server=0
fi

echo "Waiting for NameNode to be ready..."
for i in $(seq 1 30); do
  if ${COMPOSE_CMD} exec -T namenode bash -c \
       'timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report' >/dev/null 2>&1; then
    break
  fi
  [ "$i" -eq 30 ] && { echo "ERROR: NameNode did not become ready in time."; exit 1; }
  sleep 2
done

echo "Waiting for ${DN_COUNT} DataNode(s) to register..."
for i in $(seq 1 60); do
  LIVE_DNS=$(${COMPOSE_CMD} exec -T namenode bash -c \
    'timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report 2>/dev/null' \
    | grep -c "^Name:" || true)
  echo "  ${LIVE_DNS}/${DN_COUNT} DataNodes live..."
  if [ "${LIVE_DNS}" -ge "${DN_COUNT}" ]; then
    break
  fi
  [ "$i" -eq 60 ] && { echo "ERROR: Only ${LIVE_DNS}/${DN_COUNT} DataNodes registered after 2 minutes."; exit 1; }
  sleep 2
done

# Leave safe mode once DataNodes are registered
${COMPOSE_CMD} exec -T namenode bash -lc '
  export HADOOP_HOME=/opt/hadoop-3.2.1
  export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
  hdfs dfsadmin -safemode leave >/dev/null 2>&1 || true
' 2>&1 | grep -v "ttyname failed" || true

if [ "${START_HIVE}" = "1" ]; then
  echo "Preparing HDFS for Hive..."
  ${COMPOSE_CMD} exec -T namenode bash -lc '
    export HADOOP_HOME=/opt/hadoop-3.2.1
    export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
    hdfs dfs -mkdir -p /user/hive/warehouse >/dev/null 2>&1 || true
    hdfs dfs -chmod 777 /user/hive/warehouse >/dev/null 2>&1 || true
  ' 2>&1 | grep -v "ttyname failed" || true

  echo "Waiting for Hive services to be ready..."
  attempt_hive_recovery_if_needed

  wait_for_port hive-metastore hive-metastore 9083

  # Restart HiveServer2 once after HDFS path prep so it can bind cleanly.
  ${COMPOSE_CMD} restart hive-server >/dev/null 2>&1 || true

  wait_for_hiveserver2_ready
fi

echo "Cluster started: 1 NameNode + ${DN_COUNT} DataNode(s)"
echo "- NameNode UI:  http://localhost:9870"
echo "- YARN UI:      http://localhost:8088"
echo "- JobHistory:   http://localhost:19888"
if [ "${START_HIVE}" = "1" ]; then
  echo "- HiveServer2:  localhost:10000"
fi

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