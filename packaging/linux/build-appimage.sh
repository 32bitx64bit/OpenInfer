#!/usr/bin/env bash
# Build a Linux x86_64 AppImage for OpenInfer Studio.
# Downloads linuxdeploy + plugins into packaging/linux/.cache when missing.
set -euo pipefail
cd "$(dirname "$0")/../.."

VERSION="$(tr -d '[:space:]' < internal/version/VERSION)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
DATE="$(date -u +%Y-%m-%dT%H:%MZ)"
ARCH="$(uname -m)"
OUT_DIR="dist/linux"
APPDIR="$OUT_DIR/OpenInferStudio.AppDir"
CACHE="packaging/linux/.cache"
APPIMAGE="$OUT_DIR/OpenInferStudio-${VERSION}-linux-${ARCH}.AppImage"

rm -rf "$OUT_DIR"
mkdir -p "$APPDIR/usr/bin" \
         "$APPDIR/usr/share/applications" \
         "$APPDIR/usr/share/icons/hicolor/256x256/apps" \
         "$CACHE"

echo "==> building backend + desktop ($VERSION)"
./scripts/build.sh release

BIN=build/openinfer-studio
[ -f "$BIN" ] || BIN=build/apps/desktop/openinfer-studio
[ -f "$BIN" ] || { echo "desktop binary not found"; exit 1; }
cp "$BIN" "$APPDIR/usr/bin/openinfer-studio"
cp build/openinfer-core "$APPDIR/usr/bin/openinfer-core"
chmod +x "$APPDIR/usr/bin/openinfer-studio" "$APPDIR/usr/bin/openinfer-core"

# Icon (PNG preferred; fall back to SVG copy for AppDir).
ICON_SRC="packaging/icons/openinfer-studio.svg"
ICON_DST="$APPDIR/usr/share/icons/hicolor/256x256/apps/openinfer-studio.png"
if command -v rsvg-convert >/dev/null; then
  rsvg-convert -w 256 -h 256 "$ICON_SRC" -o "$ICON_DST"
elif command -v magick >/dev/null; then
  magick -background none "$ICON_SRC" -resize 256x256 "$ICON_DST"
elif command -v convert >/dev/null; then
  convert -background none "$ICON_SRC" -resize 256x256 "$ICON_DST"
else
  cp "$ICON_SRC" "$APPDIR/usr/share/icons/hicolor/256x256/apps/openinfer-studio.svg"
  ICON_DST="$APPDIR/usr/share/icons/hicolor/256x256/apps/openinfer-studio.svg"
fi

cat > "$APPDIR/usr/share/applications/openinfer-studio.desktop" <<EOF
[Desktop Entry]
Name=OpenInfer Studio
Comment=Run GGUF models locally with llama.cpp
Exec=openinfer-studio
Icon=openinfer-studio
Type=Application
Categories=Development;Utility;
Terminal=false
EOF

cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
export PATH="$HERE/usr/bin:${PATH:-}"
export LD_LIBRARY_PATH="$HERE/usr/lib:${LD_LIBRARY_PATH:-}"
export QML2_IMPORT_PATH="$HERE/usr/qml:${QML2_IMPORT_PATH:-}"
export QT_PLUGIN_PATH="$HERE/usr/plugins:${QT_PLUGIN_PATH:-}"
exec "$HERE/usr/bin/openinfer-studio" "$@"
EOF
chmod +x "$APPDIR/AppRun"

fetch() {
  local url="$1" dest="$2"
  if [ -x "$dest" ]; then return 0; fi
  echo "==> downloading $(basename "$dest")"
  curl -fsSL -o "$dest" "$url"
  chmod +x "$dest"
}

# Continuous linuxdeploy builds; pin via URL path that CI also uses.
fetch "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-${ARCH}.AppImage" \
  "$CACHE/linuxdeploy"
fetch "https://github.com/linuxdeploy/linuxdeploy-plugin-qt/releases/download/continuous/linuxdeploy-plugin-qt-${ARCH}.AppImage" \
  "$CACHE/linuxdeploy-plugin-qt"
fetch "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage" \
  "$CACHE/appimagetool"

export APPIMAGE_EXTRACT_AND_RUN=1
export QML_SOURCES_PATHS="$PWD/apps/desktop/qml"
export EXTRA_QT_PLUGINS="wayland;iconengines;imageformats;platforms;platformthemes;styles;tls;networkinformation"

echo "==> linuxdeploy (bundle Qt)"
"$CACHE/linuxdeploy" --appdir "$APPDIR" \
  --executable "$APPDIR/usr/bin/openinfer-studio" \
  --desktop-file "$APPDIR/usr/share/applications/openinfer-studio.desktop" \
  --icon-file "$ICON_DST" \
  --plugin qt

# Ensure the backend survives the deploy step.
cp -f build/openinfer-core "$APPDIR/usr/bin/openinfer-core"
chmod +x "$APPDIR/usr/bin/openinfer-core"

echo "==> appimagetool"
ARCH="$ARCH" "$CACHE/appimagetool" "$APPDIR" "$APPIMAGE"
chmod +x "$APPIMAGE"
echo "AppImage ready: $APPIMAGE (commit=$COMMIT date=$DATE)"
