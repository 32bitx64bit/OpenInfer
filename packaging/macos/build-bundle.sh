#!/usr/bin/env bash
# Package OpenInfer Studio as a macOS .app + .dmg for one architecture.
# Usage: ./packaging/macos/build-bundle.sh [arm64|x86_64]
# Default: host arch (arm64 on Apple Silicon, x86_64 on Intel).
set -euo pipefail
cd "$(dirname "$0")/../.."

HOST_ARCH="$(uname -m)"
case "${1:-}" in
  arm64|aarch64) ARCH=arm64; GOARCH=arm64 ;;
  x86_64|amd64)  ARCH=x86_64; GOARCH=amd64 ;;
  "")
    if [ "$HOST_ARCH" = "arm64" ] || [ "$HOST_ARCH" = "aarch64" ]; then
      ARCH=arm64; GOARCH=arm64
    else
      ARCH=x86_64; GOARCH=amd64
    fi
    ;;
  *)
    echo "usage: $0 [arm64|x86_64]" >&2
    exit 2
    ;;
esac

VERSION="$(tr -d '[:space:]' < internal/version/VERSION)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
DATE="$(date -u +%Y-%m-%dT%H:%MZ)"
LDFLAGS="-s -w -X github.com/openinfer/openinfer-studio/internal/version.Commit=${COMMIT} -X github.com/openinfer/openinfer-studio/internal/version.Date=${DATE}"

OUT_DIR="dist/macos-${ARCH}"
BUILD_DIR="build/macos-${ARCH}"
APP="$OUT_DIR/OpenInfer Studio.app"
DMG="$OUT_DIR/OpenInferStudio-${VERSION}-macos-${ARCH}.dmg"
STAGE="$OUT_DIR/dmg-root"

rm -rf "$OUT_DIR" "$BUILD_DIR"
mkdir -p "$OUT_DIR"

echo "==> building openinfer-core $VERSION ($ARCH)"
GOOS=darwin GOARCH="$GOARCH" CGO_ENABLED=0 \
  go build -trimpath -ldflags "$LDFLAGS" -o "$OUT_DIR/openinfer-core" ./apps/core

echo "==> building openinfer-studio ($ARCH)"
# Strip removed AGL framework refs on newer SDKs before configure/build.
if [ -n "${QT_ROOT_DIR:-}" ]; then
  while IFS= read -r -d '' f; do
    if grep -q -- '-framework AGL' "$f" 2>/dev/null; then
      sed -i '' 's/-framework AGL//g' "$f"
    fi
  done < <(find "$QT_ROOT_DIR" -type f -name '*.cmake' -print0 2>/dev/null || true)
fi

cmake -B "$BUILD_DIR" -S apps/desktop \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_OSX_ARCHITECTURES="$ARCH"
cmake --build "$BUILD_DIR" -j"$(sysctl -n hw.ncpu)"
while IFS= read -r f; do
  sed -i '' 's/-framework AGL//g' "$f"
done < <(find "$BUILD_DIR" -type f \( -name 'link.txt' -o -name 'build.ninja' -o -name 'flags.make' \) \
  -exec grep -l -- '-framework AGL' {} \; 2>/dev/null || true)
cmake --build "$BUILD_DIR" -j"$(sysctl -n hw.ncpu)"

if [ -d "$BUILD_DIR/openinfer-studio.app" ]; then
  rm -rf "$APP"
  cp -R "$BUILD_DIR/openinfer-studio.app" "$APP"
else
  mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
  cp "$BUILD_DIR/openinfer-studio" "$APP/Contents/MacOS/openinfer-studio"
fi

cp "$OUT_DIR/openinfer-core" "$APP/Contents/MacOS/openinfer-core"
chmod +x "$APP/Contents/MacOS/openinfer-studio" "$APP/Contents/MacOS/openinfer-core"

# Confirm desktop binary architecture.
file "$APP/Contents/MacOS/openinfer-studio"
file "$APP/Contents/MacOS/openinfer-core"

cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>OpenInfer Studio</string>
  <key>CFBundleDisplayName</key><string>OpenInfer Studio</string>
  <key>CFBundleIdentifier</key><string>org.openinfer.studio</string>
  <key>CFBundleVersion</key><string>${VERSION}</string>
  <key>CFBundleShortVersionString</key><string>${VERSION}</string>
  <key>CFBundleExecutable</key><string>openinfer-studio</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSMinimumSystemVersion</key><string>12.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF

echo "==> macdeployqt"
macdeployqt "$APP" -qmldir=apps/desktop/qml -always-overwrite

rm -rf "$STAGE"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
rm -f "$DMG"
hdiutil create -volname "OpenInfer Studio (${ARCH})" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
rm -rf "$STAGE"

echo "App: $APP"
echo "DMG: $DMG (commit=$COMMIT date=$DATE arch=$ARCH)"
