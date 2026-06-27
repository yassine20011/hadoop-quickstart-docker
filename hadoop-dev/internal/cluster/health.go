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

// waitFor polls fn every 2s up to maxWait.
// fn returns (done bool, progressDetail string). The detail is passed to
// m.progress() on each tick and to m.done() on success.
func waitFor(ctx context.Context, m *Manager, label string, maxWait time.Duration, fn func() (bool, string)) error {
	m.begin(label)
	deadline := time.Now().Add(maxWait)
	lastDetail := ""
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ok, detail := fn()
		lastDetail = detail
		if ok {
			m.done(label, detail)
			return nil
		}
		if detail != "" {
			m.progress(label, detail)
		}
		time.Sleep(2 * time.Second)
	}
	_ = lastDetail
	return fmt.Errorf("timeout waiting for: %s", label)
}

// waitForSimple is a convenience wrapper for checks with no progress detail.
func waitForSimple(ctx context.Context, m *Manager, label string, maxWait time.Duration, fn func() bool) error {
	return waitFor(ctx, m, label, maxWait, func() (bool, string) {
		return fn(), ""
	})
}

func waitForNameNode(ctx context.Context, m *Manager) error {
	return waitForSimple(ctx, m, "Waiting for NameNode", 60*time.Second, func() bool {
		_, err := execInContainer(ctx, m.cli, ContainerNameNode,
			[]string{"bash", "-c", "timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report"})
		return err == nil
	})
}

func waitForDataNodes(ctx context.Context, m *Manager, dnCount int) error {
	label := fmt.Sprintf("Waiting for DataNodes (%d)", dnCount)
	return waitFor(ctx, m, label, 120*time.Second, func() (bool, string) {
		out, err := execInContainer(ctx, m.cli, ContainerNameNode,
			[]string{"bash", "-c", "timeout 5 /opt/hadoop-3.2.1/bin/hdfs dfsadmin -report 2>/dev/null"})
		if err != nil {
			return false, fmt.Sprintf("0/%d", dnCount)
		}
		live := strings.Count(out, "Name:")
		return live >= dnCount, fmt.Sprintf("%d/%d", live, dnCount)
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
	label := fmt.Sprintf("Waiting for %s", ctr)
	return waitForSimple(ctx, m, label, maxWait, func() bool {
		out, err := execInContainer(ctx, m.cli, ctr,
			[]string{"bash", "-c", fmt.Sprintf("echo > /dev/tcp/%s/%s", host, port)})
		return err == nil && !strings.Contains(out, "Connection refused")
	})
}

func waitForHiveServer2(ctx context.Context, m *Manager) error {
	if err := waitForPort(ctx, m, ContainerHiveServer, "localhost", "10000", 240*time.Second); err != nil {
		return err
	}
	return waitForSimple(ctx, m, "HiveServer2 JDBC handshake", 240*time.Second, func() bool {
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
	return waitForSimple(ctx, m, "HBase RegionServer", 120*time.Second, func() bool {
		out, err := execInContainer(ctx, m.cli, ContainerHBaseMaster,
			[]string{"bash", "-c",
				`curl -sf "http://localhost:16010/jmx?qry=Hadoop:service=HBase,name=Master,sub=Server" 2>/dev/null`})
		return err == nil &&
			strings.Contains(out, `"numRegionServers" : `) &&
			!strings.Contains(out, `"numRegionServers" : 0`)
	})
}
