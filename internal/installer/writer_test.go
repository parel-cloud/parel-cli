package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func freshFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".zshrc")
	if content != "" {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func writeOpts() WriteOptions {
	return WriteOptions{
		Shell:      Shell{Kind: "zsh", Path: "ignored", OS: "darwin"},
		Snippet:    "claude-parel() { :; }",
		CLIVersion: "0.1.0",
		Now:        time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
	}
}

func TestApply_AppendsToExistingFile(t *testing.T) {
	p := freshFile(t, "export FOO=bar\nexport BAZ=qux\n")

	changed, err := Apply(p, writeOpts())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first Apply")
	}

	content, _ := os.ReadFile(p)
	got := string(content)
	if !strings.Contains(got, "export FOO=bar") {
		t.Errorf("user content was lost: %q", got)
	}
	if !strings.Contains(got, BeginMarker) || !strings.Contains(got, EndMarker) {
		t.Errorf("markers missing: %q", got)
	}
}

func TestApply_IsIdempotent(t *testing.T) {
	p := freshFile(t, "alias ll='ls -la'\n")

	if _, err := Apply(p, writeOpts()); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(p)

	changed, err := Apply(p, writeOpts())
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("Apply should be no-op when block hasn't changed")
	}

	second, _ := os.ReadFile(p)
	if string(first) != string(second) {
		t.Errorf("file mutated on no-op apply:\nfirst:%s\nsecond:%s", string(first), string(second))
	}
}

func TestApply_ReplacesExistingBlock(t *testing.T) {
	p := freshFile(t, "")

	if _, err := Apply(p, writeOpts()); err != nil {
		t.Fatal(err)
	}

	updated := writeOpts()
	updated.Snippet = "claude-parel() { echo updated; }"
	if _, err := Apply(p, updated); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(p)
	got := string(content)
	if strings.Count(got, BeginMarker) != 1 {
		t.Errorf("expected exactly one block, got %d", strings.Count(got, BeginMarker))
	}
	if !strings.Contains(got, "echo updated") {
		t.Errorf("updated snippet missing: %q", got)
	}
}

func TestRemove_DeletesBlockButKeepsRest(t *testing.T) {
	p := freshFile(t, "alias parel-debug='echo hi'\n")
	if _, err := Apply(p, writeOpts()); err != nil {
		t.Fatal(err)
	}

	changed, err := Remove(p)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	content, _ := os.ReadFile(p)
	got := string(content)
	if strings.Contains(got, BeginMarker) {
		t.Errorf("block was not removed: %q", got)
	}
	if !strings.Contains(got, "alias parel-debug") {
		t.Errorf("user content lost: %q", got)
	}
}

func TestSetBlockDisabled_RoundTripIsByteFaithful(t *testing.T) {
	p := freshFile(t, "alias unrelated='ls'\n")

	opts := writeOpts()
	opts.Snippet = "PAREL_MODEL_ID=\"qwen3-max\"\nclaude-parel() {\n  echo hi\n}"
	if _, err := Apply(p, opts); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)

	if changed, err := SetBlockDisabled(p, true); err != nil {
		t.Fatalf("disable: %v", err)
	} else if !changed {
		t.Fatal("disable should report changed")
	}

	disabled, _ := os.ReadFile(p)
	if !strings.Contains(string(disabled), DisabledLinePrefix+"PAREL_MODEL_ID=") {
		t.Errorf("PAREL_MODEL_ID line was not commented out: %q", string(disabled))
	}
	if !strings.Contains(string(disabled), DisabledLinePrefix+"claude-parel()") {
		t.Errorf("claude-parel function header was not commented: %q", string(disabled))
	}
	if !strings.Contains(string(disabled), BeginMarker) || !strings.Contains(string(disabled), EndMarker) {
		t.Errorf("markers must survive disable: %q", string(disabled))
	}
	if !strings.Contains(string(disabled), "alias unrelated='ls'") {
		t.Errorf("user content outside the block was disturbed: %q", string(disabled))
	}

	if changed, err := SetBlockDisabled(p, false); err != nil {
		t.Fatalf("enable: %v", err)
	} else if !changed {
		t.Fatal("enable should report changed")
	}

	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Errorf("round-trip not byte-faithful:\nbefore:\n%s\nafter:\n%s", string(before), string(after))
	}
}

func TestSetBlockDisabled_NoOpWhenAlreadyInRequestedState(t *testing.T) {
	p := freshFile(t, "")
	if _, err := Apply(p, writeOpts()); err != nil {
		t.Fatal(err)
	}
	// Already enabled (no prefix anywhere) → disable=false is a no-op.
	changed, err := SetBlockDisabled(p, false)
	if err != nil || changed {
		t.Errorf("expected no-op enable on already-enabled file, got changed=%v err=%v", changed, err)
	}
	// Disable then disable again is also no-op.
	if _, err := SetBlockDisabled(p, true); err != nil {
		t.Fatal(err)
	}
	changed, err = SetBlockDisabled(p, true)
	if err != nil || changed {
		t.Errorf("expected no-op disable on already-disabled file, got changed=%v err=%v", changed, err)
	}
}

func TestSetBlockDisabled_NoBlockReturnsFalse(t *testing.T) {
	p := freshFile(t, "alias x='echo'\n")
	changed, err := SetBlockDisabled(p, true)
	if err != nil || changed {
		t.Errorf("expected no-op when no managed block exists, got changed=%v err=%v", changed, err)
	}
}

func TestInspect_ReturnsCurrentBlock(t *testing.T) {
	p := freshFile(t, "")
	if _, err := Apply(p, writeOpts()); err != nil {
		t.Fatal(err)
	}
	got, err := Inspect(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "claude-parel() {") {
		t.Errorf("Inspect: %q does not contain snippet", got)
	}
}
