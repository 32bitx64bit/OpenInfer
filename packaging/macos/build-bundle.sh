#!/usr/bin/env bash
# Package OpenInfer Studio as a macOS arm64 .app + .dmg.
set -euo pipefail
cd "$(dirname "$0")/../.."

VERSION="$(tr -d '[:space:]' < internal/version/VERSION)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
DATE="$(date -u +%Y-%m-%dT%H:%MZ)"
LDFLAGS="-s -w -X github.com/openinfer/openinfer-studio/internal/version.Commit=${COMMIT} -X github.com/openinfer/openinfer-studio/internal/version.Date=${DATE}"

OUT_DIR="dist/macos"
APP="$OUT_DIR/OpenInfer Studio.app"
DMG="$OUT_DIR/OpenInferStudio-${VERSION}-macos-arm64.dmg"
STAGE="$OUT_DIR/dmg-root"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

echo "==> building openinfer-core $VERSION"
go build -trimpath -ldflags "$LDFLAGS" -o "$OUT_DIR/openinfer-core" ./apps/core

echo "==> building openinfer-studio"
cmake -B build -S apps/desktop \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_OSX_ARCHITECTURES=arm64
# Strip removed AGL framework refs on newer SDKs.
if [ -n "${QT_ROOT_DIR:-}" ]; then
  while IFS= read -r -d '' f; do
    if grep -q -- '-framework AGL' "$f" 2>/dev/null; then
      sed -i '' 's/-framework AGL//g' "$f"
    fi
  done < <(find "$QT_ROOT_DIR" -type f -name '*.cmake' -print0 2>/dev/null || true)
fi
cmake --build build -j"$(sysctl -n hw.ncpu)"
while IFS= read -r f; do
  sed -i '' 's/-framework AGL//g' "$f"
done < <(find build -type f \( -name 'link.txt' -o -name 'build.ninja' -o -name 'flags.make' \) \
  -exec grep -l -- '-framework AGL' {} \; 2>/dev/null || true)
cmake --build build -j"$(sysctl -n hw.ncpu)"

if [ -d build/openinfer-studio.app ]; then
  rm -rf "$APP"
  cp -R build/openinfer-studio.app "$APP"
else
  mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
  cp build/openinfer-studio "$APP/Contents/MacOS/openinfer-studio"
fi

cp "$OUT_DIR/openinfer-core" "$APP/Contents/MacOS/openinfer-core"
chmod +x "$APP/Contents/MacOS/openinfer-studio" "$APP/Contents/MacOS/openinfer-core"

# Refresh Info.plist with release metadata (CMake may already have written one).
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

# DMG with Applications shortcut for drag-install.
rm -rf "$STAGE"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
rm -f "$DMG"
hdiutil create -volname "OpenInfer Studio" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
rm -rf "$STAGE"

echo "App: $APP"
echo "DMG: $DMG (commit=$COMMIT date=$DATE)"
