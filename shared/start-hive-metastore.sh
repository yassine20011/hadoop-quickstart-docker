#!/bin/bash

set -e

# Initialize Hive metastore schema once (no-op if already initialized)
if ! /opt/hive/bin/schematool -info -dbType postgres >/dev/null 2>&1; then
  /opt/hive/bin/schematool -initSchema -dbType postgres
fi

exec /opt/hive/bin/hive --service metastore