# 🐘 hadoop-dev

A lightning-fast, zero-dependency CLI tool for spinning up local Hadoop development clusters. 

No 10GB VMs, no VirtualBox kernel issues, and no messy bash scripts or `docker-compose` files to manage. `hadoop-dev` is a single compiled Go binary that orchestrates everything using the Docker Engine directly.

---

## 🌟 Features

* **Single Binary**: No need to clone a repo. Just download the executable and run it. All necessary startup scripts are embedded right inside the CLI.
* **Instant Start**: Boots a complete Hadoop cluster (with NameNode, DataNodes, and optional Hive/HBase) in ~15 seconds.
* **Web Dashboard**: Features a beautiful built-in web UI to monitor containers, view streaming logs, and easily upload files into HDFS.
* **Smart Presets**: Scale DataNodes horizontally on the fly, or quickly toggle heavy services like Hive and HBase.
* **Cross-Platform**: Works flawlessly on Linux, macOS, and Windows.

---

## 🚀 Installation

1. **Download** the latest binary for your OS from the [Releases](../../releases) page.
2. **Install it globally** by running the `install` command. This will copy the binary to your system PATH so you can use it from any directory:

```bash
# Run this from wherever you downloaded the binary
./hadoop-dev install
```

*(On Windows, `hadoop-dev.exe install` will automatically add itself to your User PATH via PowerShell!)*

---

## 🛠️ Usage

### Starting the Cluster

Start a minimal Hadoop cluster (NameNode + 2 DataNodes) and attach to the interactive shell:

```bash
hadoop-dev start
```

**Presets and Scaling:**
You can customize the cluster topology using flags:
* `minimal`: Hadoop only (NameNode + DataNodes)
* `standard`: Hadoop + Hive Metastore + HiveServer2
* `full`: Hadoop + Hive + HBase Master + RegionServer

```bash
# Start a standard cluster with 3 DataNodes, but don't attach the shell
hadoop-dev start --preset standard --datanodes 3 --no-attach
```

### 🖥️ The Web Dashboard

To visually manage your cluster, view real-time logs, and drop files directly into the `shared/` folder or HDFS:

```bash
hadoop-dev web
```
This will open `http://localhost:8080` in your default browser.

### Checking Status & Logs

See which containers are running:
```bash
hadoop-dev status
```

Stream logs for any service:
```bash
# Stream NameNode logs
hadoop-dev logs -f namenode

# Stream HiveServer logs
hadoop-dev logs -f hive-server
```

### Stopping the Cluster

Tear everything down cleanly:
```bash
hadoop-dev stop
```

---

## 📂 The `shared/` Directory

When you start the cluster, `hadoop-dev` will automatically create a `shared/` folder in your current working directory. 

This folder acts as a bridge between your host machine and the Docker containers. Any file you drop in here is instantly accessible inside the containers at `/shared`.

---

## 🌐 Quick Reference: Web UIs

Once the cluster is running, the following services are mapped to `localhost`:

| Service | URL |
| --- | --- |
| **hadoop-dev Dashboard** | [http://localhost:8080](http://localhost:8080) |
| HDFS NameNode | [http://localhost:9870](http://localhost:9870) |
| YARN Resource Manager | [http://localhost:8088](http://localhost:8088) |
| Job History Server | [http://localhost:19888](http://localhost:19888) |
| HBase Master | [http://localhost:16010](http://localhost:16010) |
| HiveServer2 (JDBC) | `localhost:10000` |

---

## 🔧 Legacy Migration Notes

If you used previous versions of this project, you might remember `setup.sh`, `run.sh`, `stop.sh`, and `docker-compose.yml`. 
* **These files are gone!** 
* The new `hadoop-dev` CLI replaces all of them by directly communicating with the Docker Daemon via its Go SDK. 

---

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.
