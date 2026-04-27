package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisableMarkerPath_RespectsHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := disableMarkerPath()
	if err != nil {
		t.Fatalf("disableMarkerPath: %v", err)
	}
	want := filepath.Join(tmp, ".parel", "claude-code.disabled")
	if got != want {
		t.Errorf("path mismatch: want %q, got %q", want, got)
	}
}

func TestDisableMarkerExists_DetectsCreateAndRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	exists, _, err := disableMarkerExists()
	if err != nil {
		t.Fatalf("initial exists check: %v", err)
	}
	if exists {
		t.Fatal("expected no marker on a fresh home")
	}

	path, _ := disableMarkerPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("disabled"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	exists, gotPath, err := disableMarkerExists()
	if err != nil {
		t.Fatalf("post-create exists check: %v", err)
	}
	if !exists {
		t.Fatal("expected marker to be detected")
	}
	if gotPath != path {
		t.Errorf("path mismatch: want %q, got %q", path, gotPath)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	exists, _, err = disableMarkerExists()
	if err != nil {
		t.Fatalf("post-remove exists check: %v", err)
	}
	if exists {
		t.Fatal("expected marker to be gone after removal")
	}
}
