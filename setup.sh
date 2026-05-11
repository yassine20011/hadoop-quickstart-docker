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
if ! chmod 777 data/hive/postgresql 2>/dev/null; then
	echo "WARN: could not chmod data/hive/postgresql (likely owned by a container UID); continuing."
fi

# Make scripts executable
chmod +x run.sh
chmod +x shared/start-hadoop.sh
chmod +x shared/start-datanode.sh
chmod +x shared/start-hive-metastore.sh
chmod +x shared/compose-namenode.sh
chmod +x shared/compose-datanode.sh

# Optional: install Pig into shared/
read -r -p "Install Apache Pig 0.17.0 for scripting? [y/N]: " pig_answer
if [[ "${pig_answer}" =~ ^[Yy]$ ]]; then
	if [ -d "shared/pig-0.17.0" ]; then
		echo "Pig already installed in shared/, skipping."
	else
		echo "Downloading and extracting Apache Pig 0.17.0..."
		curl -L https://downloads.apache.org/pig/pig-0.17.0/pig-0.17.0.tar.gz | tar -xz -C shared/
		echo "✅ Pig ready at shared/pig-0.17.0"
	fi
fi

echo ""
echo "✅ Setup complete! Directory structure:"
echo ""
echo "  data/nn/      → persistent NameNode state"
echo "  data/dn1..N/  → persistent DataNode block storage (created by run.sh)"
echo "  data/hive/    → persistent Hive metastore DB"
echo "  history/      → bash history across container restarts"
echo "  hadoop.env    → Hadoop property overrides"
echo "  shared/       → drop your data files here"
echo ""
echo "👉 Run ./run.sh (you will be asked for DataNode count)"