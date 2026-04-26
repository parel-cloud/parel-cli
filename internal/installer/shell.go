package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Shell describes the active shell environment used to render and write the
// claude-parel launcher.
type Shell struct {
	Kind       string // "zsh" | "bash" | "powershell"
	ProfileFmt string // human-readable hint, e.g. "~/.zshrc"
	Path       string // absolute path to the profile file
	OS         string // runtime.GOOS at detection time
}

// IsPowerShell reports whether the snippet should be PowerShell flavor.
func (s Shell) IsPowerShell() bool { return s.Kind == "powershell" }

// SourceCommand prints the right post-install hint.
func (s Shell) SourceCommand() string {
	switch s.Kind {
	case "powershell":
		return "Restart your PowerShell session, then run: claude-parel"
	default:
		base := s.ProfileFmt
		if base == "" {
			base = s.Path
		}
		return fmt.Sprintf("Run `source %s` (or open a new terminal), then: claude-parel", base)
	}
}

// DetectShell picks the right shell + profile path for the current host.
//
//	macOS:   $SHELL=zsh -> ~/.zshrc, $SHELL=bash -> ~/.bashrc, fallback ~/.zshrc
//	Linux:   $SHELL=zsh -> ~/.zshrc, default -> ~/.bashrc
//	Windows: PowerShell $PROFILE
func DetectShell() (Shell, error) {
	switch runtime.GOOS {
	case "windows":
		return detectPowerShell()
	default:
		return detectPosix()
	}
}

func detectPosix() (Shell, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Shell{}, err
	}
	shellEnv := strings.ToLower(filepath.Base(os.Getenv("SHELL")))

	kind := "bash"
	rel := ".bashrc"
	if shellEnv == "zsh" || (shellEnv == "" && runtime.GOOS == "darwin") {
		kind = "zsh"
		rel = ".zshrc"
	}

	abs := filepath.Join(home, rel)
	pretty := "~/" + rel
	return Shell{Kind: kind, ProfileFmt: pretty, Path: abs, OS: runtime.GOOS}, nil
}

func detectPowerShell() (Shell, error) {
	// Prefer the user's OneDrive-aware Documents folder. We can't invoke
	// `$PROFILE` from Go, but the standard PowerShell 5.1+ default is:
	//   %USERPROFILE%\Documents\PowerShell\Microsoft.PowerShell_profile.ps1   (PS 7+)
	//   %USERPROFILE%\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1  (PS 5.1)
	// We write to PowerShell 7+ first; the user's running shell may need to
	// `. $PROFILE` regardless.
	home, err := os.UserHomeDir()
	if err != nil {
		return Shell{}, err
	}
	abs := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return Shell{}, err
	}
	return Shell{Kind: "powershell", ProfileFmt: "$PROFILE", Path: abs, OS: runtime.GOOS}, nil
}
