package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func withTempXDG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("HOME", dir)
	}
	return dir
}

func TestRoundTrip_DefaultProfile(t *testing.T) {
	withTempXDG(t)

	f, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(f.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %d", len(f.Profiles))
	}

	f.Set("default", Profile{APIKey: "pk-abc", BaseURL: "https://api.parel.cloud"})
	if err := f.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	g, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	prof, name := g.Resolve("")
	if name != "default" {
		t.Errorf("Resolve(\"\"): name = %q want default", name)
	}
	if prof.APIKey != "pk-abc" {
		t.Errorf("APIKey: got %q", prof.APIKey)
	}
}

func TestPath_PermissionsAreRestricted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce POSIX mode bits")
	}
	withTempXDG(t)
	f := empty()
	f.Set("default", Profile{APIKey: "pk"})
	if err := f.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := Path()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("permissions: got %o want 0600", mode)
	}
}

func TestResolve_Fallbacks(t *testing.T) {
	f := empty()
	f.Set("staging", Profile{APIKey: "pk-stg"})
	f.DefaultProfile = "staging"

	if _, name := f.Resolve(""); name != "staging" {
		t.Errorf("expected staging via default, got %q", name)
	}
	if p, name := f.Resolve("missing"); name != "missing" || p.APIKey != "" {
		t.Errorf("missing profile should return zero Profile, got %+v name=%q", p, name)
	}
}

func TestDelete_ResetsDefault(t *testing.T) {
	f := empty()
	f.Set("ci", Profile{APIKey: "pk-ci"})
	f.DefaultProfile = "ci"

	ok := f.Delete("ci")
	if !ok {
		t.Fatal("expected Delete to return true")
	}
	if f.DefaultProfile != "default" {
		t.Errorf("DefaultProfile reset: got %q want default", f.DefaultProfile)
	}

	// Sanity: file path still points inside the temp dir.
	withTempXDG(t)
	p, _ := Path()
	if !filepath.IsAbs(p) {
		t.Errorf("Path: expected absolute, got %q", p)
	}
}
