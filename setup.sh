#!/bin/bash

set -e

echo "Setting up Hadoop Docker Dev environment..."

# Create directory structure
mkdir -p data/nn
mkdir -p data/hive/postgresql
mkdir -p history
mkdir -p shared

# Create empty bash history if not exists
touch history/.bash_history

# Fix permissions (Docker runs as root)
chmod 777 data/nn
chmod 777 data/hive/postgresql

# Make scripts executable
chmod +x run.sh
chmod +x shared/start-hadoop.sh
chmod +x shared/start-datanode.sh
chmod +x shared/compose-namenode.sh
chmod +x shared/compose-datanode.sh

echo ""
echo "✅ Setup complete! Directory structure:"
echo ""
echo "  data/nn/      → persistent NameNode state"
echo "  data/hive/    → persistent Hive metastore DB"
echo "  history/      → bash history across container restarts"
echo "  hadoop.env    → Hadoop property overrides"
echo "  shared/       → drop your data files here"
echo ""
echo "👉 Run ./run.sh (you will be asked for DataNode count)"