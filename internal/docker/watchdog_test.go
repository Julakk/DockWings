package docker

import (
	"testing"
	"time"
)

func TestCrashTracker(t *testing.T) {
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var c crashTracker

	for i, want := range []bool{false, false, false, true} {
		got := c.observe(t0.Add(time.Duration(i)*15*time.Second), 5+i)
		if got != want {
			t.Fatalf("sampel %d: dapat %v, mau %v", i, got, want)
		}
	}

	// Restart tersebar di luar window nggak dihitung crash loop.
	var slow crashTracker
	slow.observe(t0, 1)
	if slow.observe(t0.Add(crashWindow+time.Minute), 4) {
		t.Fatal("restart yang tersebar lebih dari window jangan dianggap loop")
	}

	// Counter turun (container di-start manual) = riwayat dibuang.
	var reset crashTracker
	reset.observe(t0, 10)
	reset.observe(t0.Add(15*time.Second), 11)
	if reset.observe(t0.Add(30*time.Second), 0) {
		t.Fatal("counter turun harus reset riwayat")
	}
	if len(reset.samples) != 1 {
		t.Fatalf("riwayat harusnya 1 sampel, dapat %d", len(reset.samples))
	}
}

func TestParseInspect(t *testing.T) {
	out := "/dockpanel-aaa|restarting|7|unless-stopped\n" +
		"Error response from daemon: No such container: dockpanel-zzz\n" +
		"/dockpanel-bbb|running|0|no\n" +
		"/dockpanel-ccc|running|x|no\n"
	got := parseInspect(out)
	if len(got) != 2 {
		t.Fatalf("mau 2 container valid, dapat %d: %+v", len(got), got)
	}
	if got[0].name != "dockpanel-aaa" || got[0].restarts != 7 || got[0].policy != "unless-stopped" || got[0].state != "restarting" {
		t.Fatalf("parse salah: %+v", got[0])
	}
	if got[1].name != "dockpanel-bbb" || got[1].policy != "no" {
		t.Fatalf("parse salah: %+v", got[1])
	}
}
