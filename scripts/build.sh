#!/usr/bin/env bash
# OpenInfer Studio — reproducible development build.
# Usage: ./scripts/build.sh [debug|release]
set -euo pipefail

cd "$(dirname "$0")/.."
MODE="${1:-debug}"

echo "==> gofmt / vet"
gofmt -l . | tee /dev/stderr | (! read) || { echo "gofmt violations"; exit 1; }
go vet ./...

echo "==> Building backend (openinfer-core)"
VERSION="$(tr -d '[:space:]' < internal/version/VERSION)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
DATE="$(date -u +%Y-%m-%dT%H:%MZ)"
LDFLAGS="-X github.com/openinfer/openinfer-studio/internal/version.Commit=${COMMIT} -X github.com/openinfer/openinfer-studio/internal/version.Date=${DATE}"
if [ "$MODE" = "release" ]; then
    go build -trimpath -ldflags "-s -w ${LDFLAGS}" -o build/openinfer-core ./apps/core
else
    go build -ldflags "${LDFLAGS}" -o build/openinfer-core ./apps/core
fi

echo "==> Building desktop (openinfer-studio) version=${VERSION}"
BUILD_TYPE=Release
[ "$MODE" = "debug" ] && BUILD_TYPE=Debug
cmake -B build -S apps/desktop -DCMAKE_BUILD_TYPE="$BUILD_TYPE"
cmake --build build -j"$(nproc 2>/dev/null || sysctl -n hw.ncpu || echo 4)"

# Place the backend next to the desktop binary so the launcher finds it.
BIN_DIR="build"
[ -f build/openinfer-studio ] || BIN_DIR="build/apps/desktop"
if [ "$BIN_DIR" != "build" ]; then
    cp build/openinfer-core "$BIN_DIR"/openinfer-core
fi

echo "==> Done: $BIN_DIR/openinfer-studio (+ openinfer-core)"
