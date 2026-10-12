package docker

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestStartupArgs(t *testing.T) {
	cmd := `if [[ -f x ]]; then echo ok; fi`
	want := []string{"--entrypoint", "/bin/sh", "img:1", "-c", startupShell, "dockwings-startup", cmd}
	if got := startupArgs("img:1", cmd); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %v, mau %v", got, want)
	}
	if got := startupArgs("img:1", ""); !reflect.DeepEqual(got, []string{"img:1"}) {
		t.Fatalf("tanpa startup, args = %v", got)
	}
}

func TestStartupShellSupportsBashSyntax(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("tidak ada /bin/bash")
	}
	out, err := exec.Command("/bin/sh", "-c", startupShell, "dockwings-startup",
		`if [[ "a" == "a" ]]; then echo ok; fi`).CombinedOutput()
	if err != nil {
		t.Fatalf("gagal: %v (%s)", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("output = %q, mau ok", out)
	}
}
