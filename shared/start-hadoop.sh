#!/bin/bash

HADOOP_HOME="${HADOOP_HOME:-/opt/hadoop-3.2.1}"
HDFS_BIN="${HDFS_BIN:-}"
START_LOCAL_DATANODE="${START_LOCAL_DATANODE:-true}"
NAME_DIR="/tmp/hadoop-root/dfs/name"
DATA_DIR="/tmp/hadoop-root/dfs/data"
LOG_DIR="${HADOOP_HOME}/logs"

export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"

if [ -z "${HDFS_BIN}" ]; then
	if command -v hdfs >/dev/null 2>&1; then
		HDFS_BIN="$(command -v hdfs)"
	elif [ -x "${HADOOP_HOME}/bin/hdfs" ]; then
		HDFS_BIN="${HADOOP_HOME}/bin/hdfs"
	else
		echo "ERROR: hdfs binary not found (PATH=${PATH})"
		exit 1
	fi
fi

echo "Starting Hadoop..."
echo "Using hdfs binary: ${HDFS_BIN}"

mkdir -p "${NAME_DIR}/current" "${DATA_DIR}/current"

# clean stale locks (important for Docker restarts)
rm -f "${NAME_DIR}/current/"*.lock "${NAME_DIR}/current/"*.pid 2>/dev/null || true
rm -f "${DATA_DIR}/current/"*.lock "${DATA_DIR}/current/"*.pid 2>/dev/null || true

# first boot needs NameNode format
if [ ! -f "${NAME_DIR}/current/VERSION" ]; then
	echo "Formatting NameNode (first run)..."
	"${HDFS_BIN}" namenode -format -force -nonInteractive || exit 1
fi

"${HDFS_BIN}" --daemon start namenode || true

if [ "${START_LOCAL_DATANODE}" = "true" ]; then
	"${HDFS_BIN}" --daemon start datanode || true
else
	echo "Skipping local DataNode startup (START_LOCAL_DATANODE=${START_LOCAL_DATANODE})"
fi

echo "Starting YARN..."
yarn --daemon start resourcemanager || true
yarn --daemon start nodemanager || true

echo "Starting MapReduce JobHistory..."
mapred --daemon start historyserver || true

sleep 3
jps

if ! jps | grep -q "NameNode"; then
	echo "ERROR: NameNode did not start. Last NameNode logs:"
	tail -n 80 "${LOG_DIR}"/*namenode*.log 2>/dev/null || tail -n 80 /opt/hadoop-3.2.1/logs/*namenode*.log 2>/dev/null || true
	exit 1
fi

if ! jps | grep -q "ResourceManager"; then
	echo "WARNING: ResourceManager did not start. Last RM logs:"
	tail -n 60 "${LOG_DIR}"/*resourcemanager*.log 2>/dev/null || true
fi

if ! jps | grep -q "JobHistoryServer"; then
	echo "WARNING: JobHistoryServer did not start. Last JobHistory logs:"
	tail -n 60 "${LOG_DIR}"/*historyserver*.log 2>/dev/null || true
fi