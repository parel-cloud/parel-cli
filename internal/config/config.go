// Package config loads and persists Parel CLI profiles.
//
// File location:
//   POSIX:    $XDG_CONFIG_HOME/parel/auth.toml (default ~/.config/parel/auth.toml)
//   Windows:  %APPDATA%\parel\auth.toml
//
// Permissions: the file is created with mode 0600. A warning is printed if a
// world-readable copy is detected on POSIX.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const fileName = "auth.toml"

type Profile struct {
	APIKey    string    `toml:"api_key,omitempty"`
	BaseURL   string    `toml:"base_url,omitempty"`
	CreatedAt time.Time `toml:"created_at,omitempty"`
}

type File struct {
	DefaultProfile string             `toml:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles"`
}

func empty() *File {
	return &File{DefaultProfile: "default", Profiles: map[string]Profile{}}
}

// Path returns the absolute path to auth.toml. The directory is created if
// missing (mode 0700 on POSIX).
//
// Layout:
//
//	$XDG_CONFIG_HOME/parel/auth.toml      (POSIX, including macOS, mirrors gh CLI)
//	~/.config/parel/auth.toml             (POSIX fallback when XDG_CONFIG_HOME unset)
//	%APPDATA%\parel\auth.toml             (Windows)
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

func configDir() (string, error) {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "parel"), nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "parel"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "parel"), nil
}

// Load reads auth.toml. A missing file returns an empty File without error.
func Load() (*File, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty(), nil
		}
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(p); err == nil {
			if info.Mode().Perm()&0o077 != 0 {
				fmt.Fprintf(os.Stderr, "warning: %s is world- or group-readable. Run: chmod 600 %s\n", p, p)
			}
		}
	}
	f := empty()
	if err := toml.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("decode %s: %w", p, err)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	if f.DefaultProfile == "" {
		f.DefaultProfile = "default"
	}
	return f, nil
}

// Save persists the file at 0600.
func (f *File) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	buf, err := toml.Marshal(f)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Resolve picks the active profile by following the precedence:
//
//  1. Explicit `name` (from --profile or PAREL_PROFILE)
//  2. f.DefaultProfile
//
// Empty inputs fall through to "default". Returns the matching Profile (zero
// value if absent) and the resolved name.
func (f *File) Resolve(name string) (Profile, string) {
	if name == "" {
		name = f.DefaultProfile
	}
	if name == "" {
		name = "default"
	}
	p, ok := f.Profiles[name]
	if !ok {
		return Profile{}, name
	}
	return p, name
}

// Set creates or updates a profile.
func (f *File) Set(name string, p Profile) {
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC().Truncate(time.Second)
	}
	f.Profiles[name] = p
	if f.DefaultProfile == "" {
		f.DefaultProfile = name
	}
}

// Delete removes a profile. Returns false if the profile didn't exist.
func (f *File) Delete(name string) bool {
	if _, ok := f.Profiles[name]; !ok {
		return false
	}
	delete(f.Profiles, name)
	if f.DefaultProfile == name {
		f.DefaultProfile = "default"
	}
	return true
}
