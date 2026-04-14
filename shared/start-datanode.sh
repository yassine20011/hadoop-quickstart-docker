#!/bin/bash

set -e

HADOOP_HOME="${HADOOP_HOME:-/opt/hadoop-3.2.1}"
HDFS_BIN="${HADOOP_HOME}/bin/hdfs"
DATA_DIR="/tmp/hadoop-root/dfs/data"

export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"

echo "Starting DataNode in $(hostname)..."
mkdir -p "${DATA_DIR}/current"
rm -f "${DATA_DIR}/current/"*.lock "${DATA_DIR}/current/"*.pid 2>/dev/null || true

"${HDFS_BIN}" --daemon start datanode || true

sleep 2
jps

if ! jps | grep -q "DataNode"; then
  echo "ERROR: DataNode did not start"
  tail -n 80 "${HADOOP_HOME}/logs"/*datanode*.log 2>/dev/null || true
  exit 1
fi
