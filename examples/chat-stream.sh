#!/usr/bin/env bash
# Streaming chat example.
#
# Prereq: `parel auth login` (or PAREL_API_KEY env var).
set -euo pipefail

parel chat "Bana 2 satırlık bir Türkçe haiku yaz." \
  --model qwen3-max \
  --stream \
  --max-tokens 80
