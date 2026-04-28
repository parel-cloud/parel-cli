# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.5] - 2026-04-28

### Added
- `parel claude-code init --model-name "<display>"` flag lets you override the
  picker label without editing `~/.zshrc` by hand.
- When `--model-name` is omitted, the CLI now resolves a human-readable name
  from the gateway: BYOMs come from `/v1/deployments/<uuid>` (display_name →
  name → huggingface_id), platform/instant models from `/v1/models/<id>` with
  a `/v1/models` list scan as fallback. The interactive picker also stops
  discarding the display name it already fetched. Falls back to
  `"<id> (Parel)"` if every lookup misses.

### Changed
- `parel claude-code disable` now also comments out every non-marker line in
  the managed block by prefixing it with `# parel-off: `. After the user
  re-sources their profile, no `PAREL_*` shell variables and no `claude-parel`
  function definition remain — so plain `claude` stops seeing the Parel
  custom model option in the `/model` picker. `enable` strips the prefix back
  out and the block returns to its original byte-for-byte state. The marker
  file at `~/.parel/claude-code.disabled` is still maintained for instant
  in-shell fallthrough; the comment toggle is the durable layer for new
  shells.

## [0.1.4] - 2026-04-27

### Added
- `parel claude-code disable` and `parel claude-code enable` toggle the
  Parel custom model option without uninstalling the launcher. `disable`
  drops a marker at `~/.parel/claude-code.disabled`; the shell snippet now
  checks that file on every invocation and falls through to the unmodified
  `claude` binary when the marker is present. Useful when auto / agentic
  mode breaks because the chosen Parel model is misbehaving. Effect is
  instant, no shell restart needed. `enable` removes the marker.
- `parel claude-code status` reports `enabled` / `DISABLED` and prints the
  marker path. Older installed snippets without the marker check are
  detected and the user is nudged to re-run `parel claude-code init`.

### Changed
- POSIX and PowerShell `claude-parel` launcher snippets gained a marker
  short-circuit at the top of the function body. The web UI snippet
  (`web/src/components/connect-claude-code.tsx`) was updated in lockstep;
  the parity test asserts both lines appear verbatim in the TS source.

## [0.1.3] - 2026-04-27

### Changed
- All CLI prompts, help text, summary cards and shell snippet comments are
  now in English. The `claude-parel` launcher block written into the user's
  profile is also English-only. The web UI snippet remains the source of
  truth for the env-var contract; the parity test enforces only the
  literal env-var assignments.

## [0.1.2] - 2026-04-27

### Added
- `parel deployments create` is now an interactive wizard. Running the
  command with no flags walks the user through:
  1. **HF model id** — with autocomplete suggestions (top-deployed models).
     Empty input falls back to the first suggestion.
  2. **HF validate** — runs automatically; prints architecture, VRAM
     (fp16/int4), ghost test result, and recommended GPU tier.
  3. **GPU tier** — picker driven by `/v1/gpu-tiers/live`; each row shows
     VRAM, $/hr, and capacity. The recommended tier is marked `[recommended]`
     and pre-selected. Tiers smaller than the recommended VRAM get a
     `! VRAM tight` warning.
  4. **Quantization** — auto/fp16/fp8/awq/gptq picker.
  5. **Provider** (`--advanced` only) — auto smart routing or manual
     runpod/vastai/modal pick.
  6. **Idle timeout** and **Budget cap** — the budget default comes from
     the gateway preview's `hourly_cost_usd × 168 × 1.2`, falling back to $50.
  7. **Summary card** — model, GPU, ETA, hourly cost, S3 cache state;
     followed by a y/N confirmation.
- Success path prints a "Try it now" cheat sheet: `parel chat`,
  `parel claude-code init --model`, `parel proxy --upstream`.
- `--yes` flag: skips every prompt; missing flags fall back to defaults
  (CI / scripting mode).
- `--advanced` flag: surfaces the provider picker step.

### Changed
- `--idle-timeout` and `--budget` defaults are now 0. Zero triggers the
  wizard prompt or uses the preview suggestion. Old behaviour matches
  `--idle-timeout 15 --budget 100`.

## [0.1.1] - 2026-04-26

### Fixed
- `parel claude-code init` model picker now lists BYOM (dedicated) and
  imported HF instant tenant models. Previously the picker filter was too
  strict on `model_type` and silently dropped any model with an empty type
  (which is the default for BYOM).

### Changed
- Model picker is now hierarchical: the wizard first asks **My own GPUs
  (BYOM)** / **My imported models** / **Parel showcase**, then narrows
  to the chosen bucket. Single-bucket users skip the first step. Each
  picker entry carries a `[BYOM]` / `[Instant]` rosette so the source is
  obvious at a glance.

## [0.1.0] - 2026-04-26 (initial public release)

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
