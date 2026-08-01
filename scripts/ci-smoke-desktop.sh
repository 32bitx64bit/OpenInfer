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

# Prefer offscreen; fall back to minimal if the plugin is missing.
export QT_QPA_PLATFORM="${QT_QPA_PLATFORM:-offscreen}"
export QT_LOGGING_RULES="${QT_LOGGING_RULES:-*.debug=false}"

echo "==> smoke launching: $APP (platform=$QT_QPA_PLATFORM)"
"$APP" >smoke-desktop.log 2>&1 &
PID=$!

cleanup() {
  if kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    sleep 1
    kill -9 "$PID" 2>/dev/null || true
  fi
  # Windows may need taskkill when bash kill does not reap the Win32 process.
  if command -v taskkill >/dev/null 2>&1; then
    taskkill //PID "$PID" //F //T >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Give the bootstrap time to find the backend and become ready.
sleep 8

if ! kill -0 "$PID" 2>/dev/null; then
  echo "==> app exited early; log:"
  cat smoke-desktop.log || true
  wait "$PID" || true
  exit 1
fi

echo "==> smoke ok (process still running after 8s)"
exit 0
