# Bridging Claude Code through `parel proxy`

The `parel proxy` server lets you point Claude Code (or any Anthropic /
OpenAI SDK) at `http://127.0.0.1:7878` so your IDE config never holds a Parel
API key.

## One terminal: start the proxy

```bash
parel auth login          # paste your key once; saved to ~/.config/parel/auth.toml
parel proxy --port 7878
```

The proxy logs each forwarded request on stderr.

## Another terminal: run Claude Code

```bash
ANTHROPIC_BASE_URL=http://127.0.0.1:7878/anthropic \
ANTHROPIC_AUTH_TOKEN=any \
ANTHROPIC_API_KEY="" \
ANTHROPIC_CUSTOM_MODEL_OPTION=qwen3-max \
ANTHROPIC_CUSTOM_MODEL_OPTION_NAME="qwen3-max (Parel)" \
claude
```

`ANTHROPIC_AUTH_TOKEN` value is irrelevant — the proxy strips it and replaces
it with the Parel key from your profile. `ANTHROPIC_API_KEY` MUST stay empty
or Claude Code's login picker takes over.

## Pin a specific BYOM deployment

```bash
parel proxy --port 7878 --upstream byom-15b2ff2a-...
```

The proxy rewrites the `model` field of every forwarded chat request to your
BYOM deployment id, regardless of what the IDE sends.

## Health check

```bash
curl -s http://127.0.0.1:7878/healthz
```

Returns:

```json
{
  "ok": true,
  "version": "0.1.0",
  "upstream": "https://api.parel.cloud",
  "upstream_byom": ""
}
```
