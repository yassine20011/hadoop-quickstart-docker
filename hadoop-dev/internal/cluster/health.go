package cluster

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// execInContainer runs a command inside a container and returns stdout.
func execInContainer(ctx context.Context, cli *client.Client, containerName string, cmd []string) (string, error) {
	resp, err := cli.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", err
	}
	attach, err := cli.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer attach.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(attach.Reader)
	return buf.String(), nil
}

// waitFor polls fn every 2s up to maxWait, emitting step/ok events via m.
func waitFor(ctx context.Context, m *Manager, label string, maxWait time.Duration, fn func() bool) error {
	m.step(label)
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if fn() {
			m.ok(label)
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout: %s", label)
}

func waitForNameNode(ctx context.Context, m *Manager) error {
	return waitFor(ctx, m, "Waiting for NameNode to be ready...", 60*time.Second, func() bool {
		_, err := execInContainer(ctx, m.cli, ContainerNameNode,
			[]string{"bash", "-c", "timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report"})
		return err == nil
	})
}

func waitForDataNodes(ctx context.Context, m *Manager, dnCount int) error {
	label := fmt.Sprintf("Waiting for %d DataNode(s) to register...", dnCount)
	return waitFor(ctx, m, label, 120*time.Second, func() bool {
		out, err := execInContainer(ctx, m.cli, ContainerNameNode,
			[]string{"bash", "-c", "timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report 2>/dev/null"})
		if err != nil {
			return false
		}
		live := strings.Count(out, "Name:")
		fmt.Printf("    %d/%d DataNodes live...\n", live, dnCount)
		return live >= dnCount
	})
}

func leaveSafeMode(ctx context.Context, m *Manager) {
	_, _ = execInContainer(ctx, m.cli, ContainerNameNode,
		[]string{"bash", "-c",
			"export HADOOP_HOME=/opt/hadoop-3.2.1 && " +
				"export PATH=${HADOOP_HOME}/bin:${PATH} && " +
				"hdfs dfsadmin -safemode leave"})
}

func waitForPort(ctx context.Context, m *Manager, ctr, host, port string, maxWait time.Duration) error {
	label := fmt.Sprintf("Waiting for %s on %s:%s...", ctr, host, port)
	return waitFor(ctx, m, label, maxWait, func() bool {
		out, err := execInContainer(ctx, m.cli, ctr,
			[]string{"bash", "-c", fmt.Sprintf("echo > /dev/tcp/%s/%s", host, port)})
		return err == nil && !strings.Contains(out, "Connection refused")
	})
}

func waitForHiveServer2(ctx context.Context, m *Manager) error {
	if err := waitForPort(ctx, m, ContainerHiveServer, "localhost", "10000", 240*time.Second); err != nil {
		return err
	}
	return waitFor(ctx, m, "HiveServer2 JDBC handshake...", 240*time.Second, func() bool {
		_, err := execInContainer(ctx, m.cli, ContainerHiveServer,
			[]string{"bash", "-lc",
				"/opt/hive/bin/beeline -u jdbc:hive2://localhost:10000 -n hive -e '!quit' >/dev/null 2>&1"})
		return err == nil
	})
}

func waitForHBase(ctx context.Context, m *Manager) error {
	if err := waitForPort(ctx, m, ContainerHBaseMaster, "hbase-master", "16010", 120*time.Second); err != nil {
		return err
	}
	return waitFor(ctx, m, "HBase RegionServer to register...", 120*time.Second, func() bool {
		out, err := execInContainer(ctx, m.cli, ContainerHBaseMaster,
			[]string{"bash", "-c",
				`curl -sf "http://localhost:16010/jmx?qry=Hadoop:service=HBase,name=Master,sub=Server" 2>/dev/null`})
		return err == nil &&
			strings.Contains(out, `"numRegionServers" : `) &&
			!strings.Contains(out, `"numRegionServers" : 0`)
	})
}
