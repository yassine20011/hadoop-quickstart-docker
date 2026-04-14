#!/bin/bash

set -e

CONTAINER_NAME="hadoop-dev"

# remove old container if it exists
docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true

# keep container alive in background
# HDFS NameNode UI:      http://localhost:9870
# YARN ResourceManager:  http://localhost:8088
# JobHistory UI:         http://localhost:19888
docker run -d --name "${CONTAINER_NAME}" \
  -p 9870:9870 \
  -p 8088:8088 \
  -p 19888:19888 \
  -v "$(pwd)/shared:/shared" \
  -v "$(pwd)/data:/hadoop-data" \
  -v "$(pwd)/history/.bash_history:/root/.bash_history" \
  bde2020/hadoop-base:2.0.0-hadoop3.2.1-java8 \
  tail -f /dev/null >/dev/null

# start hadoop and open an interactive shell with Hadoop env
docker exec -it "${CONTAINER_NAME}" bash -lc '
  export HADOOP_HOME=/opt/hadoop-3.2.1
  export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"
  /shared/start-hadoop.sh
  exec env HADOOP_HOME="${HADOOP_HOME}" PATH="${PATH}" bash -i
'