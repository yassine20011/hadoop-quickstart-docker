#!/bin/bash

set -e

export HADOOP_HOME=/opt/hadoop-3.2.1
export PATH="${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH}"

/shared/start-hadoop.sh

exec tail -f /dev/null
