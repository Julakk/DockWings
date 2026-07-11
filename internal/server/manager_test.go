package server

import "testing"

func TestManagerAddAndGet(t *testing.T) {
	mgr := NewManager()

	s := &Server{UUID: "abc-123", Image: "alpine:3.4"}
	mgr.Add(s)

	got, err := mgr.Get("abc-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.UUID != "abc-123" {
		t.Errorf("expected uuid abc-123, got %s", got.UUID)
	}
}

func TestManagerGetNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.Get("nggak-ada")
	if err == nil {
		t.Fatal("expected error buat server yang nggak ada, tapi nil")
	}
}

func TestManagerRemove(t *testing.T) {
	mgr := NewManager()

	s := &Server{UUID: "abc-123"}
	mgr.Add(s)
	mgr.Remove("abc-123")

	_, err := mgr.Get("abc-123")
	if err == nil {
		t.Fatal("expected error setelah di-remove, tapi nil")
	}
}

func TestContainerName(t *testing.T) {
	s := &Server{UUID: "abc-123"}

	expected := "dockpanel-abc-123"
	if s.ContainerName() != expected {
		t.Errorf("expected %s, got %s", expected, s.ContainerName())
	}
}
