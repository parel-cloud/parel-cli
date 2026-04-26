<p align="center">
  <a href="https://parel.cloud">
    <img src="https://raw.githubusercontent.com/parel-cloud/parel-cli/main/assets/hero.svg" alt="Parel CLI" width="760"/>
  </a>
</p>

<h1 align="center">parel-cli</h1>

<p align="center">
  <b>The official command-line interface for <a href="https://parel.cloud">Parel</a>.</b><br/>
  Deploy BYOM models, list the catalog, run inference, and bridge any OpenAI- or Anthropic-compatible client to Parel via the local proxy.
</p>

<p align="center">
  <a href="https://github.com/parel-cloud/parel-cli/releases"><img alt="Release" src="https://img.shields.io/github/v/release/parel-cloud/parel-cli?color=%230b5fff&label=release"/></a>
  <a href="https://github.com/parel-cloud/parel-cli"><img alt="GitHub" src="https://img.shields.io/github/stars/parel-cloud/parel-cli?style=social"/></a>
  <a href="./LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/license-MIT-22c55e"/></a>
  <a href="#"><img alt="Go 1.23+" src="https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white"/></a>
</p>

<p align="center">
  <a href="https://parel.cloud"><b>parel.cloud</b></a> ·
  <a href="https://app.parel.cloud">Dashboard</a> ·
  <a href="https://docs.parel.cloud">Docs</a> ·
  <a href="https://github.com/parel-cloud/parel-cli/issues">Issues</a>
</p>

---

## Install

### macOS / Linux

```bash
brew install parel-cloud/tap/parel
```

Or one-line install:

```bash
curl -fsSL https://parel.cloud/install.sh | sh
```

### Windows

```powershell
scoop bucket add parel-cloud https://github.com/parel-cloud/scoop-bucket
scoop install parel
```

Or:

```powershell
iwr -useb https://parel.cloud/install.ps1 | iex
```

## Quickstart

```bash
# 1. Log in (paste your API key when prompted)
parel auth login

# 2. List catalog models
parel models list

# 3. One-shot chat
parel chat "Merhaba, sen kimsin?" --model qwen3-max --stream

# 4. Bridge Claude Code to Parel (5-minute setup)
parel claude-code init
source ~/.zshrc
claude-parel  # Claude Code now talks to Parel

# 5. Killer feature: local proxy
parel proxy --port 7878 &
ANTHROPIC_BASE_URL=http://127.0.0.1:7878/anthropic ANTHROPIC_AUTH_TOKEN=any claude
```

## Commands

13 command groups, ~30 subcommands. Run `parel --help` for the full list.

| Group | Highlights |
|---|---|
| `parel auth` | login / logout / whoami / status |
| `parel models` | list / show |
| `parel deployments` | create / start / stop / events --follow / billing (BYOM lifecycle) |
| `parel chat` | one-shot or interactive REPL with streaming |
| `parel images` / `videos` / `audio` / `embeddings` | inference dispatch with file output |
| `parel proxy` | local OpenAI- and Anthropic-compatible HTTP server (KILLER) |
| `parel claude-code` | init / status / uninstall — install `claude-parel` shell launcher |
| `parel usage` | summary / budget / spending / gpu-billing |
| `parel tasks` | list / show / cancel async generation tasks |
| `parel keys` | create / list / revoke API keys |
| `parel run` | dispatch wrapper for any model + auto file output |
| `parel env pull` | dump default key + model + base URL into `.env.local` |
| `parel completion` | bash / zsh / fish / powershell autocomplete |

## Configuration

Profiles live at `~/.config/parel/auth.toml` (POSIX) or `%APPDATA%\parel\auth.toml` (Windows).

Environment overrides:

| Variable | Default |
|---|---|
| `PAREL_API_KEY` | profile value |
| `PAREL_BASE_URL` | `https://api.parel.cloud` |
| `PAREL_PROFILE` | `default` |
| `NO_COLOR` | unset |

## Build from source

```bash
git clone https://github.com/parel-cloud/parel-cli
cd parel-cli
go build -o parel ./cmd/parel
./parel version
```

Requires Go 1.23+.

## License

[MIT](./LICENSE) © 2026 parel-cloud
