package docker

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// parseNetDev menjumlahkan byte masuk (rx) dan keluar (tx) semua interface
// kecuali loopback, dari isi /proc/<pid>/net/dev.
func parseNetDev(content string) (rx, tx int64) {
	for _, line := range strings.Split(content, "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		name := strings.TrimSpace(line[:i])
		if name == "" || name == "lo" {
			continue
		}
		f := strings.Fields(line[i+1:])
		if len(f) < 9 {
			continue
		}
		r, err1 := strconv.ParseInt(f[0], 10, 64)
		t, err2 := strconv.ParseInt(f[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		rx += r
		tx += t
	}
	return rx, tx
}

// containerNet baca statistik network network-namespace milik proses container.
func containerNet(pid int) (rx, tx int64) {
	if pid <= 0 {
		return 0, 0
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/net/dev")
	if err != nil {
		return 0, 0
	}
	return parseNetDev(string(b))
}

// uptimeMs hitung milidetik sejak container start dari State.StartedAt.
func uptimeMs(startedAt string, now time.Time) int64 {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(startedAt))
	if err != nil || t.IsZero() || t.After(now) {
		return 0
	}
	return now.Sub(t).Milliseconds()
}
