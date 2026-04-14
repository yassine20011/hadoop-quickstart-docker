# 🐘 Hadoop Docker Dev

A lightweight, Docker-based Hadoop development environment. No 10GB VMs, no VirtualBox kernel issues — just Docker.

> Built as a simpler alternative to the Cloudera QuickStart VM for academic and learning purposes.

---

## Why not the Cloudera QuickStart VM?

| Metric | Cloudera VM | This setup |
| --- | --- | --- |
| Size | ~10 GB | ~1 GB |
| Hadoop version | 2.6 (CDH5) | 3.2.1 |
| Startup time | ~5 min | ~15 sec |
| Kernel issues | Yes (VirtualBox) | None |
| File sharing | Painful | Just drop in `shared/` |

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed and running
- Docker Compose (`docker compose` or `docker-compose`)
- Bash (Linux/macOS) or WSL2 (Windows)

---

## Getting Started

```bash
# from project root
./setup.sh
./run.sh
```

`run.sh` now prompts for how many DataNodes to start (default: `2`).

Example:

```text
How many DataNodes do you want? [2]: 2
```

That's it — you'll land in an interactive shell inside the NameNode container with Hadoop fully running.

---

## Project Structure

```text
.
├── setup.sh                # One-time local setup (dirs + permissions)
├── run.sh                  # Compose launcher (prompts + scales DataNodes)
├── docker-compose.yml      # NameNode + scalable DataNode services
├── hadoop.env              # Hadoop config overrides (core/hdfs/yarn/mapred)
├── shared/
│   ├── start-hadoop.sh     # Starts NameNode + YARN + JobHistory
│   ├── start-datanode.sh   # Starts DataNode in DataNode containers
│   ├── compose-namenode.sh # Compose wrapper for NameNode service
│   ├── compose-datanode.sh # Compose wrapper for DataNode service
│   └── (your data files)   # Drop files here to access them in the container
└── data/
  └── nn/                   # NameNode persistent state
```

> `shared/` is your bridge: anything you place here is instantly available inside the container at `/shared`.

---

## Hadoop Config Overrides (`hadoop.env`)

You can override Hadoop properties in [hadoop.env](hadoop.env) without editing scripts.

Examples:

```env
HDFS_CONF_dfs_replication=2
HDFS_CONF_dfs_blocksize=268435456
```

Then restart:

```bash
docker compose down --remove-orphans
./run.sh
```

Tip: if you run only 1 DataNode, keep `HDFS_CONF_dfs_replication=1`.

---

## Loading Files into HDFS

```bash
# 1. Copy your file into shared/ on your laptop
cp ~/Downloads/myfile.csv ~/Desktop/hadoop/shared/

# 2. Inside the container, put it into HDFS
hdfs dfs -mkdir -p /user/root/input
hdfs dfs -put /shared/myfile.csv /user/root/input/

# 3. Verify
hdfs dfs -ls /user/root/input/
```

---

## Web UIs

Once the container is running, open these in your browser:

| UI | URL |
| --- | --- |
| HDFS NameNode | <http://localhost:9870> |
| YARN Resource Manager | <http://localhost:8088> |
| Job History Server | <http://localhost:19888> |

---

## Useful Commands

```bash
# reconnect to NameNode container
docker compose exec namenode bash

# list cluster containers
docker ps --filter name=hadoop-

# stop and remove cluster
docker compose down --remove-orphans

# start again
./run.sh

# start without attaching shell (CI / scripts)
NO_ATTACH=1 ./run.sh
```

---

## Replication Test (2+ DataNodes)

Start cluster with at least 2 DataNodes, then run:

```bash
hdfs dfs -setrep -w 2 /user/root/tp_bigdata/purchases.txt
```

Verify:

```bash
hdfs fsck /user/root/tp_bigdata/purchases.txt -files -blocks -locations
```

---

## Quick Test — WordCount

```bash
# Create input in HDFS
hdfs dfs -mkdir -p /user/root/input
echo "hello world hello hadoop" > /tmp/test.txt
hdfs dfs -put /tmp/test.txt /user/root/input/

# Run WordCount
hadoop jar $HADOOP_HOME/share/hadoop/mapreduce/hadoop-mapreduce-examples-*.jar \
  wordcount /user/root/input /user/root/output

# Check results
hdfs dfs -cat /user/root/output/part-r-00000
```

---

## Hadoop Version

| Component | Version |
| --- | --- |
| Hadoop | 3.2.1 |
| Java | 8 |
| Base Image | [bde2020/hadoop-base](https://hub.docker.com/r/bde2020/hadoop-base) |

---

## Stopping the Environment

```bash
docker compose down --remove-orphans
```

Re-running `./run.sh` will start fresh automatically.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
