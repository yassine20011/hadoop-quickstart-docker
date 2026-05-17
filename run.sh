#!/bin/bash

set -e

# ---------------------------------------------------------------------------
# Arrow-key single-select menu.
# Usage: arrow_menu "Title" option1 option2 ...
# Returns selected index in ARROW_RESULT.
# ---------------------------------------------------------------------------
arrow_menu() {
  local title="$1"; shift
  local options=("$@")
  local selected=0 key esc

  tput civis 2>/dev/null || true
  trap 'tput cnorm 2>/dev/null || true' RETURN

  while true; do
    tput cuu $((${#options[@]} + 1)) 2>/dev/null || true
    echo "  ${title}"
    for i in "${!options[@]}"; do
      if [ "$i" -eq "$selected" ]; then
        echo "  $(tput bold 2>/dev/null)> ${options[$i]}$(tput sgr0 2>/dev/null)"
      else
        echo "    ${options[$i]}"
      fi
    done

    IFS= read -rsn1 key
    if [[ "$key" == $'\x1b' ]]; then
      IFS= read -rsn2 -t 0.1 esc || true
      case "$esc" in
        '[A') [ "$selected" -gt 0 ] && selected=$(( selected - 1 )) || true ;;
        '[B') [ "$selected" -lt $(( ${#options[@]} - 1 )) ] && selected=$(( selected + 1 )) || true ;;
      esac
    elif [[ "$key" == '' ]]; then
      break
    fi
  done

  tput cnorm 2>/dev/null || true
  ARROW_RESULT=$selected
}

print_menu_placeholder() {
  local lines="$1"
  for _ in $(seq 1 "$lines"); do echo; done
}

# ---------------------------------------------------------------------------
# Resolve preset → PROFILE_FLAGS
# ---------------------------------------------------------------------------
DN_COUNT_DEFAULT=2
PROFILE_FLAGS=()

# Legacy START_HIVE compat
if [ -n "${START_HIVE:-}" ] && [ -z "${PRESET:-}" ]; then
  case "${START_HIVE,,}" in
    y|yes) PRESET="standard" ;;
    *)     PRESET="minimal"  ;;
  esac
fi

apply_preset() {
  case "$1" in
    minimal)  PROFILE_FLAGS=() ;;
    standard) PROFILE_FLAGS=("--profile" "hive") ;;
    full)     PROFILE_FLAGS=("--profile" "hive" "--profile" "hbase") ;;
    *)        echo "ERROR: unknown preset '$1'. Valid: minimal standard full custom"; exit 1 ;;
  esac
}

if [ -n "${PRESET:-}" ]; then
  apply_preset "${PRESET}"
elif [ ! -t 0 ]; then
  apply_preset "minimal"
else
  PRESET_OPTS=(
    "minimal  — Hadoop only (~1.5 GB)"
    "standard — Hadoop + Hive (~2.5 GB)"
    "full     — Hadoop + Hive + HBase (~3.5 GB)"
    "custom   — pick components"
  )
  print_menu_placeholder $(( ${#PRESET_OPTS[@]} + 1 ))
  arrow_menu "Select a preset:" "${PRESET_OPTS[@]}"
  case $ARROW_RESULT in
    0) apply_preset "minimal"  ;;
    1) apply_preset "standard" ;;
    2) apply_preset "full"     ;;
    3)
      print_menu_placeholder 3
      arrow_menu "Include Hive?" "yes" "no"
      [ "$ARROW_RESULT" -eq 0 ] && PROFILE_FLAGS+=("--profile" "hive")
      print_menu_placeholder 3
      arrow_menu "Include HBase?" "yes" "no"
      [ "$ARROW_RESULT" -eq 0 ] && PROFILE_FLAGS+=("--profile" "hbase")
      ;;
  esac
fi

# DataNode count
if [ -n "${DN_COUNT:-}" ]; then
  :
elif [ -t 0 ]; then
  read -r -p "How many DataNodes? [${DN_COUNT_DEFAULT}]: " DN_COUNT
else
  DN_COUNT="${DN_COUNT_DEFAULT}"
fi
DN_COUNT="${DN_COUNT:-${DN_COUNT_DEFAULT}}"
if ! [[ "${DN_COUNT}" =~ ^[0-9]+$ ]] || [ "${DN_COUNT}" -lt 1 ]; then
  echo "ERROR: DataNode count must be an integer >= 1"; exit 1
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
profile_active() { [[ " ${PROFILE_FLAGS[*]} " == *" $1 "* ]]; }

if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  echo "ERROR: docker compose is required."; exit 1
fi

# Generate per-DataNode compose override
DATANODES_COMPOSE="$(pwd)/docker-compose.datanodes.yml"
{
  echo "services:"
  for n in $(seq 1 "${DN_COUNT}"); do
    mkdir -p "$(pwd)/data/dn${n}"
    chmod 777 "$(pwd)/data/dn${n}"
    cat <<EOF
  datanode${n}:
    image: bde2020/hadoop-base:2.0.0-hadoop3.2.1-java8
    container_name: hadoop-datanode-${n}
    hostname: datanode${n}
    networks:
      - hadoop
    depends_on:
      - namenode
    env_file:
      - ./hadoop.env
    environment:
      CORE_CONF_fs_defaultFS: hdfs://namenode:8020
    volumes:
      - ./shared:/shared
      - ./data/dn${n}:/tmp/hadoop-root
    command:
      - bash
      - /shared/compose-datanode.sh
EOF
  done
} > "${DATANODES_COMPOSE}"

# Single array used for every compose call — no word-splitting issues
COMPOSE_ARGS=(-f docker-compose.yml -f "${DATANODES_COMPOSE}" "${PROFILE_FLAGS[@]}")

mkdir -p "$(pwd)/data/hive/postgresql" "$(pwd)/data/hbase" "$(pwd)/history"
touch "$(pwd)/history/.bash_history"
chmod 777 "$(pwd)/data/hbase" 2>/dev/null || true
chmod 777 "$(pwd)/data/hive/postgresql" 2>/dev/null || \
  echo "WARN: could not chmod data/hive/postgresql; continuing."

is_service_running() {
  ${COMPOSE_CMD} "${COMPOSE_ARGS[@]}" ps --status running --services 2>/dev/null | grep -qx "$1"
}

reset_hive_postgres_data() {
  local dir="$(pwd)/data/hive/postgresql"
  echo "Resetting Hive metastore PostgreSQL data..."
  mkdir -p "${dir}"
  docker run --rm -v "${dir}:/var/lib/postgresql/data" alpine:3.20 \
    sh -c 'rm -rf /var/lib/postgresql/data/* /var/lib/postgresql/data/.[!.]* /var/lib/postgresql/data/..?* || true'
}

attempt_hive_recovery_if_needed() {
  for i in $(seq 1 45); do
    is_service_running hive-metastore && is_service_running hive-server && return 0
    sleep 2
  done
  if ${COMPOSE_CMD} logs --no-color hive-metastore 2>/dev/null \
      | grep -qE "Version information not found in metastore|database .* does not exist|schemaTool failed"; then
    echo "WARN: Hive metastore schema invalid. Attempting recovery..."
    ${COMPOSE_CMD} "${COMPOSE_ARGS[@]}" down --remove-orphans >/dev/null 2>&1 || true
    reset_hive_postgres_data
    ${COMPOSE_CMD} "${COMPOSE_ARGS[@]}" up -d
    for i in $(seq 1 45); do
      is_service_running hive-metastore && is_service_running hive-server && { echo "Hive recovery succeeded."; return 0; }
      sleep 2
    done
    echo "ERROR: Hive recovery failed."
  else
    echo "ERROR: Hive services failed to start."
  fi
  ${COMPOSE_CMD} logs --tail 120 hive-metastore || true
  ${COMPOSE_CMD} logs --tail 120 hive-server || true
  return 1
}

wait_for_port() {
  local service="$1" host="$2" port="$3" retries="${4:-60}"
  echo "Waiting for ${service} on ${host}:${port}..."
  for i in $(seq 1 "${retries}"); do
    ${COMPOSE_CMD} exec -T "${service}" bash -c "echo > /dev/tcp/${host}/${port}" 2>/dev/null && return 0
    sleep 2
  done
  echo "ERROR: ${service} port ${port} never opened."; return 1
}

wait_for_hiveserver2_ready() {
  echo "Waiting for HiveServer2 on port 10000..."
  wait_for_port hive-server localhost 10000 120
  for i in $(seq 1 120); do
    ${COMPOSE_CMD} exec -T hive-server bash -lc \
      "/opt/hive/bin/beeline -u jdbc:hive2://localhost:10000 -n hive -e '!quit' >/dev/null 2>&1" 2>/dev/null && return 0
    sleep 2
  done
  echo "ERROR: HiveServer2 did not become ready."
  ${COMPOSE_CMD} logs --tail 120 hive-server || true
  return 1
}

wait_for_hbase_ready() {
  wait_for_port hbase-master hbase-master 16010 60
  echo "Waiting for HBase RegionServer to register..."
  for i in $(seq 1 60); do
    if ${COMPOSE_CMD} exec -T hbase-master bash -c \
      'curl -sf "http://localhost:16010/jmx?qry=Hadoop:service=HBase,name=Master,sub=Server" 2>/dev/null | grep -q "\"numRegionServers\" : [1-9]"' 2>/dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "ERROR: HBase RegionServer did not register in time."
  ${COMPOSE_CMD} logs --tail 60 hbase-master || true
  return 1
}

# ---------------------------------------------------------------------------
# Start cluster
# ---------------------------------------------------------------------------
docker rm -f hadoop-namenode hadoop-dev >/dev/null 2>&1 || true
docker ps -a --format '{{.Names}}' | grep '^hadoop-datanode-' | xargs -r docker rm -f >/dev/null 2>&1 || true

${COMPOSE_CMD} "${COMPOSE_ARGS[@]}" down --remove-orphans >/dev/null 2>&1 || true
${COMPOSE_CMD} "${COMPOSE_ARGS[@]}" up -d

# Wait for NameNode
echo "Waiting for NameNode to be ready..."
for i in $(seq 1 30); do
  ${COMPOSE_CMD} exec -T namenode bash -c \
    'timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report' >/dev/null 2>&1 && break
  [ "$i" -eq 30 ] && { echo "ERROR: NameNode did not become ready."; exit 1; }
  sleep 2
done

# Wait for DataNodes
echo "Waiting for ${DN_COUNT} DataNode(s) to register..."
for i in $(seq 1 60); do
  LIVE_DNS=$(${COMPOSE_CMD} exec -T namenode bash -c \
    'timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report 2>/dev/null' \
    | grep -c "^Name:" || true)
  echo "  ${LIVE_DNS}/${DN_COUNT} DataNodes live..."
  [ "${LIVE_DNS}" -ge "${DN_COUNT}" ] && break
  [ "$i" -eq 60 ] && { echo "ERROR: Only ${LIVE_DNS}/${DN_COUNT} DataNodes registered."; exit 1; }
  sleep 2
done

# Leave safe mode
${COMPOSE_CMD} exec -T namenode bash -lc '
  export HADOOP_HOME=/opt/hadoop-3.2.1
  export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
  hdfs dfsadmin -safemode leave >/dev/null 2>&1 || true
' 2>&1 | grep -v "ttyname failed" || true

# Hive health checks
if profile_active hive; then
  echo "Preparing HDFS for Hive..."
  ${COMPOSE_CMD} exec -T namenode bash -lc '
    export HADOOP_HOME=/opt/hadoop-3.2.1
    export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
    hdfs dfs -mkdir -p /user/hive/warehouse >/dev/null 2>&1 || true
    hdfs dfs -chmod 777 /user/hive/warehouse >/dev/null 2>&1 || true
  ' 2>&1 | grep -v "ttyname failed" || true

  echo "Waiting for Hive services..."
  attempt_hive_recovery_if_needed
  wait_for_port hive-metastore hive-metastore 9083
  ${COMPOSE_CMD} restart hive-server >/dev/null 2>&1 || true
  wait_for_hiveserver2_ready
fi

# HBase health check
if profile_active hbase; then
  echo "Waiting for HDFS safe mode to clear before HBase init..."
  for i in $(seq 1 30); do
    if ${COMPOSE_CMD} exec -T namenode bash -c \
      'timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -safemode get 2>/dev/null | grep -q "Safe mode is OFF"'; then
      break
    fi
    ${COMPOSE_CMD} exec -T namenode bash -c \
      'timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -safemode leave >/dev/null 2>&1' || true
    sleep 2
  done
  wait_for_hbase_ready
fi

# ---------------------------------------------------------------------------
# Cluster summary
# ---------------------------------------------------------------------------
SUMMARY="1 NameNode + ${DN_COUNT} DataNode(s)"
profile_active hive  && SUMMARY+=" + Hive"
profile_active hbase && SUMMARY+=" + HBase"
echo "Cluster started: ${SUMMARY}"
echo "- NameNode UI:  http://localhost:9870"
echo "- YARN UI:      http://localhost:8088"
echo "- JobHistory:   http://localhost:19888"
profile_active hive  && echo "- HiveServer2:  localhost:10000"
profile_active hbase && echo "- HBase UI:     http://localhost:16010"

# ---------------------------------------------------------------------------
# Attach interactive shell
# ---------------------------------------------------------------------------
if [ "${NO_ATTACH:-0}" = "1" ]; then
  echo "NO_ATTACH=1 set, skipping shell attach."
else
  ${COMPOSE_CMD} exec namenode bash -lc '
    export HADOOP_HOME=/opt/hadoop-3.2.1
    export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
    exec env HADOOP_HOME="${HADOOP_HOME}" PATH="${PATH}" bash -i
  '
fi
