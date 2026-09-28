package docker

import (
	"context"

	"github.com/Julakk/DockWings/internal/server"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Resources adalah snapshot utilisasi container saat ini.
type Resources struct {
	State       string
	CPUAbsolute float64 // persen, ex: 12.34
	MemoryBytes int64
	DiskBytes   int64
}

var byteSizeRe = regexp.MustCompile(`^([0-9]*\.?[0-9]+)\s*([a-zA-Z]*)$`)

// parseByteSize ngubah string kayak "512MiB", "1.2GB", "0B" jadi bytes.
func parseByteSize(s string) (int64, error) {
	m := byteSizeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("format ukuran nggak dikenal: %q", s)
	}

	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}

	var mult float64
	switch strings.ToLower(m[2]) {
	case "b", "":
		mult = 1
	case "kb":
		mult = 1000
	case "kib":
		mult = 1024
	case "mb":
		mult = 1000 * 1000
	case "mib":
		mult = 1024 * 1024
	case "gb":
		mult = 1000 * 1000 * 1000
	case "gib":
		mult = 1024 * 1024 * 1024
	case "tb":
		mult = 1000 * 1000 * 1000 * 1000
	case "tib":
		mult = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unit nggak dikenal: %q", m[2])
	}

	return int64(val * mult), nil
}

// parseCPUPercent ngubah "12.34%" jadi 12.34.
func parseCPUPercent(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%")), 64)
}

// parseMemUsageBytes ngubah "10.5MiB / 512MiB" (output docker stats) jadi bytes yang KEPAKE.
func parseMemUsageBytes(s string) (int64, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("format mem usage nggak dikenal: %q", s)
	}
	return parseByteSize(parts[0])
}

// mapDockerState nyamain state Docker ke istilah yang dipahami Panel.
func mapDockerState(raw string) string {
	switch strings.TrimSpace(raw) {
	case "running":
		return "running"
	case "restarting":
		return "starting"
	case "paused", "exited", "dead", "created":
		return "offline"
	default:
		return "unknown"
	}
}

// Resources ambil state, CPU%, memory, dan disk usage container ini.
// Disk dihitung dari folder data di host (bukan exec ke dalam container),
// jadi tetap kebaca walau container lagi mati.
func (e *DockerEnvironment) Resources(ctx context.Context, s *server.Server) (Resources, error) {
	res := Resources{State: "unknown"}

	stateOut, err := run(ctx, "inspect", "-f", "{{.State.Status}}", s.ContainerName())
	if err != nil {
		res.State = "offline"
	} else {
		res.State = mapDockerState(stateOut)
	}

	if res.State == "running" {
		statOut, err := run(ctx, "stats", "--no-stream", "--format", "{{.CPUPerc}}\t{{.MemUsage}}", s.ContainerName())
		if err == nil {
			parts := strings.SplitN(statOut, "\t", 2)
			if len(parts) == 2 {
				if cpu, err := parseCPUPercent(parts[0]); err == nil {
					res.CPUAbsolute = cpu
				}
				if mem, err := parseMemUsageBytes(parts[1]); err == nil {
					res.MemoryBytes = mem
				}
			}
		}
	}

	dir := filepath.Join(e.dataDir, s.UUID)
	if out, err := exec.CommandContext(ctx, "du", "-sb", dir).Output(); err == nil {
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			if v, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				res.DiskBytes = v
			}
		}
	}

	return res, nil
}
