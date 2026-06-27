package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hadoop-dev/internal/config"
	"hadoop-dev/internal/output"
	"hadoop-dev/internal/scripts"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Manager is the main orchestrator. It holds an authenticated Docker client.
type Manager struct {
	cli    *client.Client
	emitFn func(kind, msg string)
	pr     *output.Printer // nil when called from web server
}

// NewManager creates a Manager connected to the local Docker daemon.
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Manager{cli: cli}, nil
}

func (m *Manager) Close() { _ = m.cli.Close() }

// WithPrinter returns a shallow copy of the Manager with a terminal printer set.
func (m *Manager) WithPrinter(pr *output.Printer) *Manager {
	return &Manager{cli: m.cli, emitFn: m.emitFn, pr: pr}
}

// WithEmitter returns a shallow copy of the Manager with a progress emitter set.
// The emitter receives (kind, msg) where kind is "step", "ok", "warn", "error", or "done".
func (m *Manager) WithEmitter(fn func(kind, msg string)) *Manager {
	return &Manager{cli: m.cli, emitFn: fn, pr: m.pr}
}

// ColorStatus returns a color-coded status string for the `status` table.
// Pass disableColor=true (from --no-color flag or NO_COLOR env) to get plain text.
func ColorStatus(state, status string, disableColor bool) string {
	if disableColor || os.Getenv("NO_COLOR") != "" {
		return status
	}
	const (
		green = "\033[32m"
		yellow = "\033[33m"
		red   = "\033[31m"
		reset = "\033[0m"
	)
	switch state {
	case "running":
		return green + status + reset
	case "restarting", "created":
		return yellow + status + reset
	default:
		return red + status + reset
	}
}

// ─── Internal output helpers ──────────────────────────────────────────────

func (m *Manager) begin(label string) {
	if m.pr != nil {
		m.pr.Begin(label)
	}
	if m.emitFn != nil {
		m.emitFn("step", label)
	}
}

func (m *Manager) done(label, detail string) {
	if m.pr != nil {
		m.pr.Done(label, detail)
	}
	if m.emitFn != nil {
		m.emitFn("ok", label)
	}
}

func (m *Manager) warn(msg string) {
	if m.pr != nil {
		m.pr.Warn(msg)
	}
	if m.emitFn != nil {
		m.emitFn("warn", msg)
	}
}

func (m *Manager) progress(label, detail string) {
	if m.pr != nil {
		m.pr.Progress(label, detail)
	}
}

func (m *Manager) sub(msg string) {
	if m.pr != nil {
		m.pr.Sub(msg)
	}
}

// ─── Start ────────────────────────────────────────────────────────────────

func (m *Manager) Start(ctx context.Context, cfg Config) error {
	startedAt := time.Now()

	// Resolve absolute work directory
	workDir, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return fmt.Errorf("resolve work-dir: %w", err)
	}

	// Extract essential scripts to the shared directory
	if err := scripts.ExtractTo(workDir); err != nil {
		return err
	}
	historyDir := filepath.Join(workDir, "history")
	if err := ensureDir(historyDir); err != nil {
		return err
	}
	historyFile := filepath.Join(historyDir, ".bash_history")
	if err := ensureFile(historyFile); err != nil {
		return err
	}

	// Parse hadoop.env
	envPath := filepath.Join(workDir, "hadoop.env")
	hadoopEnv, err := config.ParseHadoopEnv(envPath)
	if err != nil {
		m.warn(fmt.Sprintf("hadoop.env not found at %s — using empty config", envPath))
		hadoopEnv = nil
	}

	if m.pr != nil {
		m.pr.Header(fmt.Sprintf("Starting Hadoop cluster (preset: %s, datanodes: %d)", cfg.Preset, cfg.DNCount))
	}

	// Stop any existing cluster first
	m.begin("Cluster stopped")
	_ = m.stopSilent(ctx)
	m.done("Cluster stopped", "")

	// Create Docker network
	if err := m.ensureNetwork(ctx); err != nil {
		return err
	}

	// Create all named volumes
	for _, volName := range AllVolumes(cfg.DNCount) {
		if err := m.ensureVolume(ctx, volName); err != nil {
			return err
		}
	}

	// Start NameNode
	label := "Starting NameNode"
	m.begin(label)
	if err := m.runContainer(ctx, NameNodeSpec(workDir, hadoopEnv)); err != nil {
		return fmt.Errorf("start NameNode: %w", err)
	}
	m.done(label, "")

	// Start DataNodes
	for i := 1; i <= cfg.DNCount; i++ {
		label := fmt.Sprintf("Starting DataNode %d", i)
		m.begin(label)
		spec := DataNodeSpec(i, workDir, hadoopEnv)
		if err := m.runContainer(ctx, spec); err != nil {
			return fmt.Errorf("start DataNode %d: %w", i, err)
		}
		m.done(label, "")
	}

	// Wait for NameNode
	if err := waitForNameNode(ctx, m); err != nil {
		return err
	}

	// Wait for DataNodes
	if err := waitForDataNodes(ctx, m, cfg.DNCount); err != nil {
		return err
	}

	// Leave safe mode
	m.begin("Leaving HDFS safe mode")
	leaveSafeMode(ctx, m)
	m.done("Leaving HDFS safe mode", "")

	// Hive services
	if cfg.Preset.IncludesHive() {
		m.begin("Preparing HDFS directories for Hive")
		_, _ = execInContainer(ctx, m.cli, ContainerNameNode, []string{"bash", "-lc",
			"export HADOOP_HOME=/opt/hadoop-3.2.1 && export PATH=${HADOOP_HOME}/bin:${PATH} && " +
				"hdfs dfs -mkdir -p /user/hive/warehouse && hdfs dfs -chmod 777 /user/hive/warehouse"})
		m.done("Preparing HDFS directories for Hive", "")

		if err := m.runContainer(ctx, PostgresSpec()); err != nil {
			return fmt.Errorf("start Postgres: %w", err)
		}
		if err := m.runContainer(ctx, HiveMetastoreSpec(workDir, hadoopEnv)); err != nil {
			return fmt.Errorf("start Hive Metastore: %w", err)
		}
		if err := m.runContainer(ctx, HiveServerSpec(hadoopEnv)); err != nil {
			return fmt.Errorf("start HiveServer2: %w", err)
		}

		if err := waitForPort(ctx, m, ContainerHiveMetastore, "hive-metastore", "9083", 120*time.Second); err != nil {
			return err
		}
		if err := waitForHiveServer2(ctx, m); err != nil {
			return err
		}
	}

	// HBase services
	if cfg.Preset.IncludesHBase() {
		if err := m.runContainer(ctx, HBaseMasterSpec()); err != nil {
			return fmt.Errorf("start HBase Master: %w", err)
		}
		if err := m.runContainer(ctx, HBaseRegionServerSpec()); err != nil {
			return fmt.Errorf("start HBase RegionServer: %w", err)
		}
		if err := waitForHBase(ctx, m); err != nil {
			return err
		}
	}

	// Summary
	if m.pr != nil {
		m.pr.Elapsed(time.Since(startedAt))
	}
	m.printSummary(cfg)
	if m.emitFn != nil {
		m.emitFn("done", "Cluster ready")
	}

	// Optionally attach to NameNode interactive shell
	if !cfg.NoAttach {
		m.sub("Attaching to NameNode shell (Ctrl+D or 'exit' to leave)...")
		fmt.Fprintln(os.Stderr)
		return m.attachShell(ctx)
	}
	return nil
}

// ─── Stop ─────────────────────────────────────────────────────────────────

// Stop tears down all cluster containers and the network.
// It is silent — callers decide what to print.
func (m *Manager) Stop(ctx context.Context) error {
	return m.stopSilent(ctx)
}

func (m *Manager) stopSilent(ctx context.Context) error {
	allNames := AllContainerNames(10)
	for _, name := range allNames {
		_ = m.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
	}
	_ = m.cli.NetworkRemove(ctx, NetworkName)
	return nil
}

// ─── Status ───────────────────────────────────────────────────────────────

func (m *Manager) Status(ctx context.Context) error {
	_, err := m.ListContainers(ctx)
	return err
}

// ─── Logs ─────────────────────────────────────────────────────────────────

func (m *Manager) Logs(ctx context.Context, service string, follow bool) error {
	containerName := service
	switch service {
	case "namenode":
		containerName = ContainerNameNode
	case "hive-metastore", "hive-server", "postgres", "hbase-master", "hbase-regionserver":
		containerName = service
	}

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
		Tail:       "100",
	}
	reader, err := m.cli.ContainerLogs(ctx, containerName, opts)
	if err != nil {
		return fmt.Errorf("fetch logs for %s: %w", containerName, err)
	}
	defer reader.Close()

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
	return err
}

// ─── Internal helpers ─────────────────────────────────────────────────────

func (m *Manager) ensureNetwork(ctx context.Context) error {
	nets, err := m.cli.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", NetworkName)),
	})
	if err != nil {
		return err
	}
	for _, n := range nets {
		if n.Name == NetworkName {
			return nil
		}
	}
	_, err = m.cli.NetworkCreate(ctx, NetworkName, network.CreateOptions{
		Driver: "bridge",
	})
	return err
}

func (m *Manager) ensureVolume(ctx context.Context, name string) error {
	_, err := m.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name:   name,
		Driver: "local",
	})
	return err
}

func (m *Manager) runContainer(ctx context.Context, spec ServiceSpec) error {
	imageName := spec.ContainerCfg.Image
	_, _, err := m.cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		label := fmt.Sprintf("Pulling %s", imageName)
		m.begin(label)
		reader, pullErr := m.cli.ImagePull(ctx, imageName, image.PullOptions{})
		if pullErr != nil {
			if m.pr != nil {
				m.pr.Fail(label)
			}
			return fmt.Errorf("pull image %s: %w", imageName, pullErr)
		}
		defer reader.Close()
		_, _ = io.Copy(io.Discard, reader)
		m.done(label, "")
	}

	created, err := m.cli.ContainerCreate(ctx,
		spec.ContainerCfg,
		spec.HostCfg,
		spec.NetworkCfg,
		nil,
		spec.Name,
	)
	if err != nil {
		return err
	}

	return m.cli.ContainerStart(ctx, created.ID, container.StartOptions{})
}

func (m *Manager) attachShell(ctx context.Context) error {
	execCfg := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd: []string{"bash", "-lc",
			"export HADOOP_HOME=/opt/hadoop-3.2.1 && " +
				"export PATH=${HADOOP_HOME}/bin:${HADOOP_HOME}/sbin:${PATH} && " +
				"exec bash -i"},
	}
	resp, err := m.cli.ContainerExecCreate(ctx, ContainerNameNode, execCfg)
	if err != nil {
		return err
	}
	attach, err := m.cli.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return err
	}
	defer attach.Close()

	go func() { _, _ = io.Copy(attach.Conn, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, attach.Reader)
	return nil
}

func (m *Manager) printSummary(cfg Config) {
	parts := []string{fmt.Sprintf("1 NameNode + %d DataNode(s)", cfg.DNCount)}
	if cfg.Preset.IncludesHive() {
		parts = append(parts, "Hive")
	}
	if cfg.Preset.IncludesHBase() {
		parts = append(parts, "HBase")
	}
	fmt.Fprintf(os.Stderr, "  Cluster: %s\n\n", strings.Join(parts, " + "))

	fmt.Println("  🌐 NameNode UI:       http://localhost:9870")
	fmt.Println("  🌐 YARN UI:           http://localhost:8088")
	fmt.Println("  🌐 JobHistory UI:     http://localhost:19888")
	if cfg.Preset.IncludesHive() {
		fmt.Println("  🌐 HiveServer2 JDBC: localhost:10000")
	}
	if cfg.Preset.IncludesHBase() {
		fmt.Println("  🌐 HBase Master UI:  http://localhost:16010")
	}
	fmt.Println()
	fmt.Println("  hadoop-dev status     — show container health")
	fmt.Println("  hadoop-dev logs -f    — stream NameNode logs")
	fmt.Println("  hadoop-dev stop       — tear everything down")
}

// ─── OS helpers ───────────────────────────────────────────────────────────

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func ensureFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// ─── Public helpers for web server ────────────────────────────────────────

// ListContainers returns all hadoop-related containers (running or stopped).
func (m *Manager) ListContainers(ctx context.Context) ([]container.Summary, error) {
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("name", "hadoop-"),
			filters.Arg("name", "hive-"),
			filters.Arg("name", "hbase-"),
			filters.Arg("name", "postgres"),
		),
	})
	if err != nil {
		return nil, err
	}

	var filtered []container.Summary
	for _, c := range containers {
		if len(c.Names) == 0 {
			continue
		}
		name := strings.TrimPrefix(c.Names[0], "/")
		if name == "hadoop-namenode" ||
			name == "postgres" ||
			name == "hive-metastore" ||
			name == "hive-server" ||
			name == "hbase-master" ||
			name == "hbase-regionserver" ||
			strings.HasPrefix(name, "hadoop-datanode-") {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

// LogStream opens a read-closer for the container's stdout+stderr log stream.
func (m *Manager) LogStream(ctx context.Context, containerName string) (io.ReadCloser, error) {
	return m.cli.ContainerLogs(ctx, containerName, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       "50",
	})
}

// Exec runs a command inside a container and returns combined output.
func (m *Manager) Exec(ctx context.Context, containerName string, cmd []string) (string, error) {
	return execInContainer(ctx, m.cli, containerName, cmd)
}
