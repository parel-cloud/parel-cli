# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - TBD (initial public release)

### Added
- Repository scaffold with Cobra-based CLI skeleton (`parel`).
- Typed gateway HTTP client (`internal/client`) with OpenAI-compatible error
  envelope parsing, exponential-backoff retry on 502/503/504, SSE streaming
  parser, and 14 endpoint wrappers covering models, deployments, inference,
  images, videos, audio, embeddings, tasks, usage, and keys.
- 13 command groups, ~30 subcommands:
  - `auth` (login/logout/whoami/status)
  - `models` (list/show with `--type`, `--ready`, `--search` filters)
  - `deployments` (full BYOM lifecycle + `events --follow`, `metrics`,
    `billing`, `preview`, `validate-hf`, `templates`)
  - `gpu tiers` (catalog + `--live` capacity)
  - `chat` (one-shot + REPL with `--stream`)
  - `images`, `videos`, `audio` (speak/transcribe/voices), `embeddings`
  - `run` (dispatch wrapper that sniffs model → endpoint)
  - `proxy` — KILLER feature: local OpenAI- and Anthropic-compatible HTTP
    server forwarding to the Parel gateway with the profile key already
    attached. Optional `--upstream byom-<uuid>` rewrites the request body's
    `model` field for forced BYOM routing.
  - `claude-code init/status/uninstall` — installs the `claude-parel` shell
    launcher (zsh/bash/PowerShell) into the user's profile with an idempotent
    marker-bounded block.
  - `usage` (summary/budget/spending/gpu-billing)
  - `tasks` (list/show `--follow`/cancel)
  - `keys` (create/list/revoke)
  - `env pull` (Vercel-style `.env.local` writer with PAREL/OPENAI/ANTHROPIC
    aliases)
  - `completion` for bash, zsh, fish, PowerShell
- XDG-aware config (`~/.config/parel/auth.toml`, mode 0600) with profile
  resolver respecting `--profile` flag and `PAREL_PROFILE` env.
- Goreleaser pipeline producing 5 binaries (darwin amd64/arm64, linux
  amd64/arm64, windows amd64), brew tap auto-PR (`parel-cloud/homebrew-tap`),
  scoop bucket auto-PR (`parel-cloud/scoop-bucket`), GPG-signed checksums.
- `scripts/install.sh` and `scripts/install.ps1` one-line installers; both
  clear the macOS Gatekeeper quarantine so v0.1 unsigned binaries don't
  trigger the "developer cannot be verified" prompt.
- GitHub Actions: `ci.yml` (3-OS matrix), `release.yml` (tag-driven
  goreleaser), `e2e.yml` (nightly smoke against the live gateway).

### Known limitations (deferred to v0.1.x / v0.2)
- macOS binaries are NOT yet Apple-notarized — `xattr -d com.apple.quarantine`
  is applied automatically by the installer scripts and brew post_install.
  Notarized builds land in v0.1.1 once Apple Developer credentials (PAR-24)
  are provisioned.
- Windows binaries are NOT code-signed. SmartScreen warning is acceptable
  per the v0.1 scope; an OV cert is on the v0.2 roadmap.
- Auth flow is API-key paste only. Browser device-code flow lands in v0.2
  alongside the gateway-side device-code endpoint.

[Unreleased]: https://github.com/parel-cloud/parel-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/parel-cloud/parel-cli/releases/tag/v0.1.0
