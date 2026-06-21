package cluster

import "fmt"

// Preset represents a cluster configuration preset.
type Preset int

const (
	PresetMinimal  Preset = iota // Hadoop NameNode + DataNodes only
	PresetStandard               // + Hive (Postgres + Metastore + Server)
	PresetFull                   // + HBase Master + RegionServer
)

func ParsePreset(s string) (Preset, error) {
	switch s {
	case "minimal":
		return PresetMinimal, nil
	case "standard":
		return PresetStandard, nil
	case "full":
		return PresetFull, nil
	default:
		return PresetMinimal, fmt.Errorf("unknown preset %q — valid: minimal, standard, full", s)
	}
}

func (p Preset) String() string {
	switch p {
	case PresetStandard:
		return "standard"
	case PresetFull:
		return "full"
	default:
		return "minimal"
	}
}

func (p Preset) IncludesHive() bool  { return p >= PresetStandard }
func (p Preset) IncludesHBase() bool { return p >= PresetFull }

// Config holds everything needed to start the cluster.
type Config struct {
	WorkDir  string
	Preset   Preset
	DNCount  int
	NoAttach bool
}
