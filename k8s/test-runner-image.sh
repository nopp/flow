#!/usr/bin/env bash
set -euo pipefail

CONTAINER_CLI="${CONTAINER_CLI:-docker}"
RUNNER_IMAGE="${RUNNER_IMAGE:-localhost:5001/noppflow-runner:test}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

require_cmd "${CONTAINER_CLI}"

echo "[1/2] Building runner image: ${RUNNER_IMAGE}"
"${CONTAINER_CLI}" build -t "${RUNNER_IMAGE}" -f Dockerfile.runner .

echo "[2/2] Validating required binaries in runner image"
"${CONTAINER_CLI}" run --rm --entrypoint /bin/sh "${RUNNER_IMAGE}" -lc '
  set -eu

  required_bins="sh bash git ssh curl tar gzip kubectl helm kaniko-executor apk"
  for b in $required_bins; do
    if ! command -v "$b" >/dev/null 2>&1; then
      echo "missing required binary: $b" >&2
      exit 1
    fi
  done

  kubectl version --client >/dev/null
  helm version >/dev/null
  kaniko-executor version >/dev/null

  # Must always work: same command used by app ensure_kubectl step
  if ! command -v kubectl >/dev/null 2>&1 && command -v apk >/dev/null 2>&1; then apk add --no-cache kubectl; fi
  command -v kubectl >/dev/null 2>&1

  echo "runner image validation OK"
'
