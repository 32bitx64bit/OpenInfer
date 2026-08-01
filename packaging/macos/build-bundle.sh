#!/usr/bin/env bash
# Package OpenInfer Studio as a macOS .app bundle + DMG-ready directory.
set -euo pipefail
cd "$(dirname "$0")/../.."

VERSION="0.1.0"
APP="dist/OpenInfer Studio.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" \
    -o "$APP/Contents/MacOS/openinfer-core" ./apps/core

cmake -B build -S apps/desktop -DCMAKE_BUILD_TYPE=Release
cmake --build build -j"$(sysctl -n hw.ncpu)"
cp build/openinfer-studio "$APP/Contents/MacOS/openinfer-studio"

cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>OpenInfer Studio</string>
  <key>CFBundleDisplayName</key><string>OpenInfer Studio</string>
  <key>CFBundleIdentifier</key><string>org.openinfer.studio</string>
  <key>CFBundleVersion</key><string>${VERSION}</string>
  <key>CFBundleExecutable</key><string>openinfer-studio</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF

# Deploy Qt frameworks when macdeployqt is available.
if command -v macdeployqt >/dev/null; then
    macdeployqt "$APP" -qmldir=apps/desktop/qml
fi

echo "Bundle ready: $APP (create DMG with: hdiutil create -srcfolder '$APP' dist/OpenInferStudio-${VERSION}.dmg)"
