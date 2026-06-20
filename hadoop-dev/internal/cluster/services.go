package cluster

import (
	"fmt"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// ─── Image names ────────────────────────────────────────────────────────────

const (
	ImageHadoopBase   = "bde2020/hadoop-base:2.0.0-hadoop3.2.1-java8"
	ImageHive         = "bde2020/hive:2.3.2-postgresql-metastore"
	ImageHivePostgres = "bde2020/hive-metastore-postgresql:2.3.0"
	ImageHBaseMaster  = "bde2020/hbase-master:1.0.0-hbase1.2.6"
	ImageHBaseRegion  = "bde2020/hbase-regionserver:1.0.0-hbase1.2.6"
)

// ─── Container names ────────────────────────────────────────────────────────

const (
	ContainerNameNode     = "hadoop-namenode"
	ContainerPostgres     = "postgres"
	ContainerHiveMetastore = "hive-metastore"
	ContainerHiveServer   = "hive-server"
	ContainerHBaseMaster  = "hbase-master"
	ContainerHBaseRegion  = "hbase-regionserver"
	NetworkName           = "hadoop"
)

func DataNodeContainerName(n int) string { return fmt.Sprintf("hadoop-datanode-%d", n) }

// ─── Volume names ────────────────────────────────────────────────────────────
// All persistent storage uses named Docker volumes — works identically on
// Windows, macOS, and Linux with no path translation issues.

const (
	VolumeNameNodeData = "hadoop-nn-data"
	VolumeHivePostgres = "hadoop-hive-postgresql"
	VolumeHBaseData    = "hadoop-hbase-data"
)

func DataNodeVolumeName(n int) string { return fmt.Sprintf("hadoop-dn%d-data", n) }

// AllVolumes returns every named volume the cluster may need.
func AllVolumes(dnCount int) []string {
	vols := []string{VolumeNameNodeData, VolumeHivePostgres, VolumeHBaseData}
	for i := 1; i <= dnCount; i++ {
		vols = append(vols, DataNodeVolumeName(i))
	}
	return vols
}

// AllContainerNames returns every container name the cluster may create.
func AllContainerNames(dnCount int) []string {
	names := []string{
		ContainerNameNode,
		ContainerPostgres,
		ContainerHiveMetastore,
		ContainerHiveServer,
		ContainerHBaseMaster,
		ContainerHBaseRegion,
	}
	for i := 1; i <= dnCount; i++ {
		names = append(names, DataNodeContainerName(i))
	}
	return names
}

// ─── ServiceSpec ─────────────────────────────────────────────────────────────

// ServiceSpec groups everything needed to create and start a container.
type ServiceSpec struct {
	Name        string
	ContainerCfg *container.Config
	HostCfg     *container.HostConfig
	NetworkCfg  *network.NetworkingConfig
}

func hadoopNetwork() *network.NetworkingConfig {
	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			NetworkName: {},
		},
	}
}

// ─── NameNode ─────────────────────────────────────────────────────────────

func NameNodeSpec(workDir string, hadoopEnv []string) ServiceSpec {
	env := mergeEnv(hadoopEnv,
		"CORE_CONF_fs_defaultFS=hdfs://namenode:8020",
		"START_LOCAL_DATANODE=false",
	)
	ports, bindings := mustPortMap(map[string]string{
		"9870": "9870",
		"8088": "8088",
		"19888": "19888",
	})
	return ServiceSpec{
		Name: ContainerNameNode,
		ContainerCfg: &container.Config{
			Image:        ImageHadoopBase,
			Hostname:     "namenode",
			Env:          env,
			ExposedPorts: ports,
			Cmd:          []string{"bash", "/shared/compose-namenode.sh"},
		},
		HostCfg: &container.HostConfig{
			PortBindings: bindings,
			Mounts: []mount.Mount{
				bindMount(filepath.Join(workDir, "shared"), "/shared"),
				bindMount(filepath.Join(workDir, "history", ".bash_history"), "/root/.bash_history"),
				namedVolume(VolumeNameNodeData, "/tmp/hadoop-root"),
			},
			NetworkMode: container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── DataNode ─────────────────────────────────────────────────────────────

func DataNodeSpec(n int, workDir string, hadoopEnv []string) ServiceSpec {
	env := mergeEnv(hadoopEnv,
		"CORE_CONF_fs_defaultFS=hdfs://namenode:8020",
	)
	return ServiceSpec{
		Name: DataNodeContainerName(n),
		ContainerCfg: &container.Config{
			Image:    ImageHadoopBase,
			Hostname: fmt.Sprintf("datanode%d", n),
			Env:      env,
			Cmd:      []string{"bash", "/shared/compose-datanode.sh"},
		},
		HostCfg: &container.HostConfig{
			Mounts: []mount.Mount{
				bindMount(filepath.Join(workDir, "shared"), "/shared"),
				namedVolume(DataNodeVolumeName(n), "/tmp/hadoop-root"),
			},
			NetworkMode: container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── Postgres (Hive metastore DB) ─────────────────────────────────────────

func PostgresSpec() ServiceSpec {
	return ServiceSpec{
		Name: ContainerPostgres,
		ContainerCfg: &container.Config{
			Image:    ImageHivePostgres,
			Hostname: "postgres",
			Env:      []string{"POSTGRES_DB=hive"},
		},
		HostCfg: &container.HostConfig{
			Mounts:      []mount.Mount{namedVolume(VolumeHivePostgres, "/var/lib/postgresql/data")},
			NetworkMode: container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── Hive Metastore ───────────────────────────────────────────────────────

func HiveMetastoreSpec(workDir string, hadoopEnv []string) ServiceSpec {
	env := mergeEnv(hadoopEnv,
		"CORE_CONF_fs_defaultFS=hdfs://namenode:8020",
		"HIVE_SITE_CONF_hive_metastore_uris=thrift://hive-metastore:9083",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionURL=jdbc:postgresql://postgres:5432/metastore",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionDriverName=org.postgresql.Driver",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionUserName=hive",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionPassword=hive",
		"SERVICE_PRECONDITION=namenode:9870 postgres:5432",
	)
	ports, bindings := mustPortMap(map[string]string{"9083": "9083"})
	return ServiceSpec{
		Name: ContainerHiveMetastore,
		ContainerCfg: &container.Config{
			Image:        ImageHive,
			Hostname:     "hive-metastore",
			Env:          env,
			ExposedPorts: ports,
			Cmd:          []string{"bash", "/shared/start-hive-metastore.sh"},
		},
		HostCfg: &container.HostConfig{
			PortBindings: bindings,
			Mounts:       []mount.Mount{bindMount(filepath.Join(workDir, "shared"), "/shared")},
			NetworkMode:  container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── HiveServer2 ──────────────────────────────────────────────────────────

func HiveServerSpec(hadoopEnv []string) ServiceSpec {
	env := mergeEnv(hadoopEnv,
		"CORE_CONF_fs_defaultFS=hdfs://namenode:8020",
		"HIVE_SITE_CONF_hive_metastore_uris=thrift://hive-metastore:9083",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionURL=jdbc:postgresql://postgres:5432/metastore",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionDriverName=org.postgresql.Driver",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionUserName=hive",
		"HIVE_SITE_CONF_javax_jdo_option_ConnectionPassword=hive",
		"SERVICE_PRECONDITION=namenode:9870 hive-metastore:9083",
	)
	ports, bindings := mustPortMap(map[string]string{"10000": "10000", "10002": "10002"})
	return ServiceSpec{
		Name: ContainerHiveServer,
		ContainerCfg: &container.Config{
			Image:        ImageHive,
			Hostname:     "hive-server",
			Env:          env,
			ExposedPorts: ports,
		},
		HostCfg: &container.HostConfig{
			PortBindings: bindings,
			NetworkMode:  container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── HBase Master ─────────────────────────────────────────────────────────

func HBaseMasterSpec() ServiceSpec {
	ports, bindings := mustPortMap(map[string]string{
		"16010": "16010",
		"16000": "16000",
		"2181":  "2181",
	})
	return ServiceSpec{
		Name: ContainerHBaseMaster,
		ContainerCfg: &container.Config{
			Image:    ImageHBaseMaster,
			Hostname: "hbase-master",
			Env: []string{
				"HBASE_CONF_hbase_rootdir=hdfs://namenode:8020/hbase",
				"HBASE_CONF_hbase_zookeeper_quorum=hbase-master",
				"HBASE_CONF_hbase_zookeeper_property_dataDir=/tmp/zookeeper",
				"HBASE_CONF_hbase_cluster_distributed=true",
				"HBASE_MANAGES_ZK=true",
				"SERVICE_PRECONDITION=namenode:9870",
				"JAVA_OPTS=-Xmx512m",
			},
			ExposedPorts: ports,
			Entrypoint:   []string{"bash", "-c"},
			Cmd: []string{
				"/entrypoint.sh && /opt/hbase-1.2.6/bin/hbase-daemon.sh start zookeeper && exec /opt/hbase-1.2.6/bin/hbase master start",
			},
		},
		HostCfg: &container.HostConfig{
			PortBindings: bindings,
			NetworkMode:  container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── HBase RegionServer ────────────────────────────────────────────────────

func HBaseRegionServerSpec() ServiceSpec {
	ports, bindings := mustPortMap(map[string]string{"16030": "16030", "16020": "16020"})
	return ServiceSpec{
		Name: ContainerHBaseRegion,
		ContainerCfg: &container.Config{
			Image:    ImageHBaseRegion,
			Hostname: "hbase-regionserver",
			Env: []string{
				"HBASE_CONF_hbase_rootdir=hdfs://namenode:8020/hbase",
				"HBASE_CONF_hbase_zookeeper_quorum=hbase-master",
				"HBASE_CONF_hbase_regionserver_hostname=hbase-regionserver",
				"HBASE_CONF_hbase_cluster_distributed=true",
				"SERVICE_PRECONDITION=hbase-master:16010",
				"JAVA_OPTS=-Xmx512m",
			},
			ExposedPorts: ports,
		},
		HostCfg: &container.HostConfig{
			PortBindings: bindings,
			Mounts:       []mount.Mount{namedVolume(VolumeHBaseData, "/hbase/data")},
			NetworkMode:  container.NetworkMode(NetworkName),
		},
		NetworkCfg: hadoopNetwork(),
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func mergeEnv(base []string, extra ...string) []string {
	merged := make([]string, len(base), len(base)+len(extra))
	copy(merged, base)
	return append(merged, extra...)
}

func bindMount(src, dst string) mount.Mount {
	return mount.Mount{Type: mount.TypeBind, Source: src, Target: dst}
}

func namedVolume(name, dst string) mount.Mount {
	return mount.Mount{Type: mount.TypeVolume, Source: name, Target: dst}
}

func mustPortMap(pairs map[string]string) (nat.PortSet, nat.PortMap) {
	portSet := nat.PortSet{}
	portMap := nat.PortMap{}
	for containerPort, hostPort := range pairs {
		p := nat.Port(containerPort + "/tcp")
		portSet[p] = struct{}{}
		portMap[p] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}
	}
	return portSet, portMap
}
