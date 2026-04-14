# 🐘 Hadoop Docker Dev

A lightweight, Docker-based Hadoop development environment. No 10GB VMs, no VirtualBox kernel issues — just Docker.

> Built as a simpler alternative to the Cloudera QuickStart VM for academic and learning purposes.

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed and running
- Bash (Linux/macOS) or WSL2 (Windows)

---

## Getting Started

```bash
# from project root
./setup.sh
./run.sh
```

That's it — you'll land in an interactive shell inside the container with Hadoop fully running.

---

## Project Structure

```text
.
├── setup.sh                # One-time local setup (dirs + permissions)
├── run.sh                  # Main entry point — starts + attaches to container
├── shared/
│   ├── start-hadoop.sh     # Starts NameNode + DataNode inside container
│   └── (your data files)   # Drop files here to access them in the container
└── data/
  └── dfs/                # Persistent HDFS state (gitignored)
```

> `shared/` is your bridge: anything you place here is instantly available inside the container at `/shared`.

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
# reconnect to running container
docker exec -it hadoop-dev bash

# stop and remove container
docker rm -f hadoop-dev

# start again
./run.sh
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
docker rm -f hadoop-dev
```

Re-running `./run.sh` will start fresh automatically.

---

## Troubleshooting

- If you see `docker: invalid reference format`, check for inline comments after `\` in multi-line docker commands.
- If `hdfs` is not found, exit and run `./run.sh` again so shell env vars are reloaded.
- If NameNode fails to start but DataNode starts, ensure the data volume is mounted to `/tmp/hadoop-root` (not another path).

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

## License

MIT License. See [LICENSE](LICENSE) for details.
