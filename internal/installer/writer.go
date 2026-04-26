package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"time"
)

// Markers delimit the parel-managed block inside the user's shell profile.
// Keeping them stable across versions is critical for the idempotent re-run
// behavior; never rename them.
const (
	BeginMarker = "# >>> parel claude-code >>>"
	EndMarker   = "# <<< parel claude-code <<<"
)

// blockPattern matches an existing parel-managed block (greedy across lines).
var blockPattern = regexp.MustCompile(`(?ms)\n?` + regexp.QuoteMeta(BeginMarker) + `.*?` + regexp.QuoteMeta(EndMarker) + `\n?`)

// WriteOptions configures Render+Apply.
type WriteOptions struct {
	Shell    Shell
	Snippet  string
	CLIVersion string
	Now      time.Time // optional override for tests
}

// Block returns the marker-wrapped string that will be inserted into the
// user's profile. It includes a "Last updated" comment so users see when the
// block was generated and by which CLI version.
func Block(opts WriteOptions) string {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC().Truncate(time.Second)
	}
	cliVersion := opts.CLIVersion
	if cliVersion == "" {
		cliVersion = "dev"
	}
	header := fmt.Sprintf(
		"%s\n# Managed by `parel claude-code` — do not edit manually.\n# Last updated: %s by parel-cli/%s",
		BeginMarker,
		now.Format("2006-01-02"),
		cliVersion,
	)
	return strings.Join([]string{header, opts.Snippet, EndMarker}, "\n")
}

// Apply writes (or replaces in-place) the parel-managed block inside path.
// The file is created if missing. Existing non-managed content is preserved
// verbatim.
func Apply(path string, opts WriteOptions) (changed bool, err error) {
	block := Block(opts)

	original, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return false, readErr
	}

	var next []byte
	if blockPattern.Match(original) {
		next = blockPattern.ReplaceAll(original, []byte("\n"+block+"\n"))
	} else if len(original) == 0 {
		next = []byte(block + "\n")
	} else {
		// Append with one separating blank line.
		needsLF := !strings.HasSuffix(string(original), "\n")
		sep := "\n\n"
		if needsLF {
			sep = "\n\n"
		} else if strings.HasSuffix(string(original), "\n\n") {
			sep = ""
		}
		next = append(append([]byte{}, original...), []byte(sep+block+"\n")...)
	}

	if string(next) == string(original) {
		return false, nil
	}

	tmp := path + ".parel.tmp"
	if err := os.WriteFile(tmp, next, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, err
	}
	return true, nil
}

// Remove deletes the parel-managed block. Returns true if the file changed.
func Remove(path string) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !blockPattern.Match(original) {
		return false, nil
	}
	next := blockPattern.ReplaceAll(original, []byte("\n"))
	if string(next) == string(original) {
		return false, nil
	}
	tmp := path + ".parel.tmp"
	if err := os.WriteFile(tmp, next, 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}

// Inspect returns the current parel-managed block (without markers) or "".
func Inspect(path string) (string, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	loc := blockPattern.FindIndex(original)
	if loc == nil {
		return "", nil
	}
	chunk := strings.TrimSpace(string(original[loc[0]:loc[1]]))
	chunk = strings.TrimPrefix(chunk, BeginMarker)
	chunk = strings.TrimSuffix(chunk, EndMarker)
	return strings.TrimSpace(chunk), nil
}
