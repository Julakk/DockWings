package docker

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"
)

// Pengaman crash-loop. Container dibuat dengan --restart=unless-stopped biar
// server nyala lagi setelah reboot host, tapi server yang mati sendiri (EULA
// belum diterima, config salah, crash) jadi muter terus dan makan CPU. Watchdog
// ini ngehentiin container yang restart crashMax kali dalam crashWindow.
const (
	watchInterval = 10 * time.Second
	crashWindow   = 10 * time.Minute
	crashMax      = 3
)

type sample struct {
	at    time.Time
	count int
}

// crashTracker nyimpen riwayat RestartCount satu container.
type crashTracker struct {
	samples []sample
}

// observe nyatet RestartCount terbaru. Balikin true kalau container sudah
// restart crashMax kali atau lebih dalam crashWindow.
func (c *crashTracker) observe(now time.Time, count int) bool {
	if n := len(c.samples); n > 0 && count < c.samples[n-1].count {
		c.samples = nil // counter di-reset Docker (container di-start manual)
	}
	c.samples = append(c.samples, sample{at: now, count: count})

	cut := 0
	for cut < len(c.samples)-1 && now.Sub(c.samples[cut].at) > crashWindow {
		cut++
	}
	c.samples = c.samples[cut:]

	return count-c.samples[0].count >= crashMax
}

type containerInfo struct {
	name     string
	state    string
	restarts int
	policy   string
}

// parseInspect baca output `docker inspect -f '{{.Name}}|{{.State.Status}}|
// {{.RestartCount}}|{{.HostConfig.RestartPolicy.Name}}'`. Baris lain
// (mis. pesan error container yang sudah hilang) diabaikan.
func parseInspect(out string) []containerInfo {
	var res []containerInfo
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(line), "|")
		if len(f) != 4 {
			continue
		}
		n, err := strconv.Atoi(f[2])
		if err != nil {
			continue
		}
		res = append(res, containerInfo{
			name:     strings.TrimPrefix(f[0], "/"),
			state:    f[1],
			restarts: n,
			policy:   f[3],
		})
	}
	return res
}

// RunCrashWatchdog ngecek semua container server tiap watchInterval. Jalanin
// lewat `go env.RunCrashWatchdog()`; fungsi ini nggak pernah return.
func (e *DockerEnvironment) RunCrashWatchdog() {
	trackers := map[string]*crashTracker{}
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for range ticker.C {
		e.watchOnce(trackers)
	}
}

func (e *DockerEnvironment) watchOnce(trackers map[string]*crashTracker) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	out, _ := run(ctx, "ps", "-a", "--filter", "name=^dockpanel-", "--format", "{{.Names}}")
	var names []string
	for _, n := range strings.Fields(out) {
		if !strings.HasSuffix(n, "-old") { // "-old" = cadangan sementara Reconfigure
			names = append(names, n)
		}
	}

	var infos []containerInfo
	if len(names) > 0 {
		args := append([]string{"inspect", "-f",
			"{{.Name}}|{{.State.Status}}|{{.RestartCount}}|{{.HostConfig.RestartPolicy.Name}}"}, names...)
		// Error di sini (container hilang di antara ps dan inspect) nggak fatal:
		// baris container lain tetap kebaca.
		text, _ := run(ctx, args...)
		infos = parseInspect(text)
	}

	seen := map[string]bool{}
	for _, c := range infos {
		seen[c.name] = true
		if c.policy != "unless-stopped" {
			delete(trackers, c.name)
			continue
		}
		t := trackers[c.name]
		if t == nil {
			t = &crashTracker{}
			trackers[c.name] = t
		}
		if !t.observe(time.Now(), c.restarts) {
			continue
		}

		log.Printf("crash loop: %s restart %d kali dalam %s, dihentikan (Start dari panel buat nyalain lagi)",
			c.name, crashMax, crashWindow)
		if _, err := run(ctx, "update", "--restart=no", c.name); err != nil {
			log.Printf("watchdog: %v", err)
		}
		if _, err := run(ctx, "stop", "-t", "10", c.name); err != nil {
			log.Printf("watchdog: %v", err)
		}
		delete(trackers, c.name)
	}
	for name := range trackers {
		if !seen[name] {
			delete(trackers, name)
		}
	}
}
