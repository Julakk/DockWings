package docker

import (
	"testing"
	"time"
)

const sampleNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:     500       5    0    0    0     0          0         0      500       5    0    0    0     0       0          0
  eth0:    1000      10    0    0    0     0          0         0     2000      20    0    0    0     0       0          0
  eth1:     300       3    0    0    0     0          0         0      400       4    0    0    0     0       0          0
`

func TestParseNetDev(t *testing.T) {
	rx, tx := parseNetDev(sampleNetDev)
	if rx != 1300 || tx != 2400 {
		t.Fatalf("rx=%d tx=%d, harusnya 1300 dan 2400 (lo nggak dihitung)", rx, tx)
	}
	if rx, tx := parseNetDev("sampah\n"); rx != 0 || tx != 0 {
		t.Fatalf("input sampah harusnya 0, dapat %d %d", rx, tx)
	}
}

func TestUptimeMs(t *testing.T) {
	now := time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC)
	if got := uptimeMs("2026-10-09T05:59:30.5Z", now); got != 29500 {
		t.Fatalf("dapat %d, harusnya 29500", got)
	}
	for _, in := range []string{"0001-01-01T00:00:00Z", "bukan waktu", "", "2027-01-01T00:00:00Z"} {
		if got := uptimeMs(in, now); got != 0 {
			t.Fatalf("%q harusnya 0, dapat %d", in, got)
		}
	}
}
