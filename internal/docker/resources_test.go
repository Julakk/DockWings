package docker

import "testing"

func TestParseByteSize(t *testing.T) {
	cases := map[string]int64{
		"512MiB":  512 * 1024 * 1024,
		"1GiB":    1024 * 1024 * 1024,
		"10.5MiB": int64(10.5 * 1024 * 1024),
		"0B":      0,
	}
	for in, want := range cases {
		got, err := parseByteSize(in)
		if err != nil {
			t.Fatalf("parseByteSize(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("parseByteSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseCPUPercent(t *testing.T) {
	got, err := parseCPUPercent("12.34%")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 12.34 {
		t.Errorf("got %v want 12.34", got)
	}
}

func TestParseMemUsageBytes(t *testing.T) {
	got, err := parseMemUsageBytes("10MiB / 512MiB")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := int64(10 * 1024 * 1024)
	if got != want {
		t.Errorf("got %d want %d", got, want)
	}
}

func TestMapDockerState(t *testing.T) {
	if mapDockerState("running") != "running" {
		t.Error("running mismatch")
	}
	if mapDockerState("exited") != "offline" {
		t.Error("exited mismatch")
	}
	if mapDockerState("restarting") != "starting" {
		t.Error("restarting mismatch")
	}
}
