#!/usr/bin/env bash
# Streaming chat example.
#
# Prereq: `parel auth login` (or PAREL_API_KEY env var).
set -euo pipefail

parel chat "Write me a two-line haiku about the sea." \
  --model qwen3-max \
  --stream \
  --max-tokens 80
