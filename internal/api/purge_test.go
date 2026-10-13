package api

import (
	"os"
	"path/filepath"
	"testing"
)

const purgeTestUUID = "2e4e1cf8-9f5b-4046-a9a1-7366e81ff6c8"

func TestPurgeServerDataRemovesFolderButNotSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "penting.txt")
	if err := os.WriteFile(secret, []byte("jangan dihapus"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, purgeTestUUID)
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "tautan")); err != nil {
		t.Fatal(err)
	}

	removed, err := purgeServerData(root, purgeTestUUID)
	if err != nil || !removed {
		t.Fatalf("removed=%v err=%v, mau true dan nil", removed, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("folder server masih ada: %v", err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Errorf("isi di luar folder ikut terhapus lewat symlink: %v", err)
	}
}

func TestPurgeServerDataMissingFolderIsNotAnError(t *testing.T) {
	removed, err := purgeServerData(t.TempDir(), purgeTestUUID)
	if err != nil || removed {
		t.Fatalf("removed=%v err=%v, mau false dan nil", removed, err)
	}
}

func TestPurgeServerDataRejectsBadUUID(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "korban")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", ".", "..", "../korban", "korban", purgeTestUUID + "/..", `..\korban`} {
		if removed, err := purgeServerData(root, bad); err == nil || removed {
			t.Errorf("uuid %q harusnya ditolak (removed=%v err=%v)", bad, removed, err)
		}
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("folder korban ikut terhapus: %v", err)
	}
}

func TestPurgeServerDataRefusesSymlinkedServerDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "data.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, purgeTestUUID)); err != nil {
		t.Fatal(err)
	}

	if removed, err := purgeServerData(root, purgeTestUUID); err == nil || removed {
		t.Fatalf("symlink folder server harusnya ditolak (removed=%v err=%v)", removed, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "data.txt")); err != nil {
		t.Errorf("isi target symlink ikut terhapus: %v", err)
	}
}
