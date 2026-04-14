#!/bin/bash

echo "Setting up Hadoop Docker Dev environment..."

# Create directory structure
mkdir -p data/dfs/name
mkdir -p data/dfs/data
mkdir -p history
mkdir -p shared

# Create empty bash history if not exists
touch history/.bash_history

# Fix permissions (Docker runs as root)
chmod 777 data/dfs/name
chmod 777 data/dfs/data

# Make scripts executable
chmod +x run.sh
chmod +x shared/start-hadoop.sh

echo ""
echo "✅ Setup complete! Directory structure:"
echo ""
echo "  data/         → persistent HDFS state"
echo "  history/      → bash history across container restarts"
echo "  shared/       → drop your data files here"
echo ""
echo "👉 Run ./run.sh to start Hadoop"