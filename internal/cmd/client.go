package cmd

import (
	"errors"
	"fmt"

	"github.com/parel-cloud/parel-cli/internal/client"
	"github.com/parel-cloud/parel-cli/internal/config"
)

// resolveClient builds an *client.Client from the active profile + global
// flag overrides. It is the single source of truth for "which key + base URL
// am I using right now?" and used by every authenticated command.
func resolveClient() (*client.Client, string, error) {
	f, err := config.Load()
	if err != nil {
		return nil, "", err
	}

	profile, profileName := f.Resolve(flagProfile)

	apiKey := flagAPIKey
	if apiKey == "" {
		apiKey = profile.APIKey
	}
	baseURL := flagBaseURL
	if baseURL == "" {
		baseURL = profile.BaseURL
	}
	if baseURL == "" {
		baseURL = client.DefaultBaseURL
	}

	if apiKey == "" {
		return nil, profileName, errors.New("no API key configured. Run `parel auth login` or set PAREL_API_KEY")
	}

	c := client.New(client.Options{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Verbose: flagVerbose > 1,
		Version: version,
	})
	return c, profileName, nil
}

// printError renders a user-friendly message for typed gateway errors and
// returns an exit-code hint via fmt.Errorf so cobra propagates it.
func printError(err error) error {
	if err == nil {
		return nil
	}
	if pe, ok := client.AsParelError(err); ok {
		switch {
		case pe.IsAuth():
			return fmt.Errorf("auth failed: %s\nRun `parel auth login` to re-authenticate", pe.Message)
		case pe.IsRateLimit():
			suffix := ""
			if pe.RetryAfter > 0 {
				suffix = fmt.Sprintf(" (retry after %ds)", pe.RetryAfter)
			}
			return fmt.Errorf("rate limited%s: %s", suffix, pe.Message)
		case pe.IsBudgetExceeded():
			return fmt.Errorf("budget exceeded: %s", pe.Message)
		case pe.IsCapacityExhausted():
			return fmt.Errorf("no GPU capacity: %s\nTry a different gpu_tier or wait a few minutes", pe.Message)
		case pe.IsDeploymentNotReady():
			return fmt.Errorf("deployment not ready: %s", pe.Message)
		case pe.IsTimeout():
			return fmt.Errorf("timed out: %s", pe.Message)
		case pe.IsNotFound():
			return fmt.Errorf("not found: %s", pe.Message)
		default:
			return fmt.Errorf("parel error %d: %s", pe.StatusCode, pe.Message)
		}
	}
	return err
}
