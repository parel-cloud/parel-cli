#!/usr/bin/env bash
# End-to-end BYOM deployment example.
set -euo pipefail

HF=${HF:-Qwen/Qwen2.5-7B-Instruct}
GPU=${GPU:-rtx-4090}

echo "1) HF preflight..."
parel deployments validate-hf "$HF"

echo
echo "2) Cost + ETA preview..."
parel deployments preview --hf-id "$HF" --gpu "$GPU"

echo
echo "3) Creating deployment (will wait until running)..."
parel deployments create \
  --hf-id "$HF" \
  --gpu "$GPU" \
  --idle-timeout 30 \
  --budget 10 \
  --wait
