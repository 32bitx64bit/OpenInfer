#!/usr/bin/env bash
# CI smoke: launch the desktop app briefly and require that it stays alive.
# Full GUI automation is not practical in Actions; this catches missing
# backends, broken Qt deploy, and bootstrap crashes.
set -euo pipefail

APP="${1:-}"
if [ -z "$APP" ] || [ ! -e "$APP" ]; then
  echo "usage: $0 <path-to-openinfer-studio>"
  exit 2
fi

# Match the app's Fusion style; avoid native styles under headless CI.
export QT_QUICK_CONTROLS_STYLE="${QT_QUICK_CONTROLS_STYLE:-Fusion}"
export QT_LOGGING_RULES="${QT_LOGGING_RULES:-*.debug=false}"

# Prefer offscreen. On Darwin, offscreen + Quick has historically been flaky;
# try offscreen first, then minimal if the process dies immediately.
platforms=("${QT_QPA_PLATFORM:-offscreen}")
if [ "$(uname -s)" = "Darwin" ] && [ -z "${QT_QPA_PLATFORM:-}" ]; then
  platforms=(offscreen minimal)
fi

smoke_once() {
  local platform="$1"
  export QT_QPA_PLATFORM="$platform"
  rm -f smoke-desktop.log
  echo "==> smoke launching: $APP (platform=$platform style=$QT_QUICK_CONTROLS_STYLE)"
  "$APP" >smoke-desktop.log 2>&1 &
  local pid=$!

  # Give the bootstrap time to find the backend and become ready.
  sleep 8

  if kill -0 "$pid" 2>/dev/null; then
    echo "==> smoke ok (process still running after 8s)"
    kill "$pid" 2>/dev/null || true
    sleep 1
    kill -9 "$pid" 2>/dev/null || true
    if command -v taskkill >/dev/null 2>&1; then
      taskkill //PID "$pid" //F //T >/dev/null 2>&1 || true
    fi
    return 0
  fi

  echo "==> app exited early under platform=$platform; log:"
  cat smoke-desktop.log || true
  wait "$pid" 2>/dev/null || true
  return 1
}

for p in "${platforms[@]}"; do
  if smoke_once "$p"; then
    exit 0
  fi
done

echo "==> smoke failed on all platforms: ${platforms[*]}"
exit 1
