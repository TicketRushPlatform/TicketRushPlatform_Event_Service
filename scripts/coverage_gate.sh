#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <module_path> [threshold] [package ...]"
  exit 1
fi

MODULE_PATH="$1"
THRESHOLD="${2:-90}"
shift || true
shift || true

if [[ $# -gt 0 ]]; then
  PACKAGES=("$@")
else
  PACKAGES=("internal/...")
fi

cd "${MODULE_PATH}"

TEST_TARGETS=()
for pkg in "${PACKAGES[@]}"; do
  TEST_TARGETS+=("./${pkg}")
done

echo "Running coverage gate in ${MODULE_PATH}"
echo "Packages: ${TEST_TARGETS[*]}"
echo "Threshold: ${THRESHOLD}%"

go test "${TEST_TARGETS[@]}" -covermode=atomic -coverprofile=coverage.ci.out

COVER_OUTPUT="$(go tool cover -func=coverage.ci.out)"
echo "${COVER_OUTPUT}"

TOTAL="$(echo "${COVER_OUTPUT}" | awk '/^total:/ {gsub("%","",$3); print $3}')"
if [[ -z "${TOTAL}" ]]; then
  echo "failed to parse total coverage"
  exit 1
fi

awk -v total="${TOTAL}" -v threshold="${THRESHOLD}" 'BEGIN {
  if (total + 0 < threshold + 0) {
    printf("coverage gate failed: %.1f%% < %.1f%%\n", total, threshold)
    exit 1
  }
  printf("coverage gate passed: %.1f%% >= %.1f%%\n", total, threshold)
}'
