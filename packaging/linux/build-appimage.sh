#!/usr/bin/env bash
# Build a portable Linux package (AppDir; convert to AppImage with
# appimagetool if available). Requires: linuxdeployqt or manual Qt libs.
set -euo pipefail
cd "$(dirname "$0")/../.."

OUT=dist/linux/OpenInferStudio.AppDir
rm -rf dist/linux
mkdir -p "$OUT/usr/bin" "$OUT/usr/share/applications" "$OUT/usr/share/icons/hicolor/256x256"

./scripts/build.sh release

BIN=build/openinfer-studio
[ -f "$BIN" ] || BIN=build/apps/desktop/openinfer-studio
cp "$BIN" "$OUT/usr/bin/openinfer-studio"
cp build/openinfer-core "$OUT/usr/bin/openinfer-core"

cat > "$OUT/usr/share/applications/openinfer-studio.desktop" <<'EOF'
[Desktop Entry]
Name=OpenInfer Studio
Comment=Run GGUF models locally with llama.cpp
Exec=openinfer-studio
Icon=openinfer-studio
Type=Application
Categories=Development;AI;
EOF

cat > "$OUT/AppRun" <<'EOF'
#!/bin/sh
exec "$(dirname "$0")/usr/bin/openinfer-studio" "$@"
EOF
chmod +x "$OUT/AppRun"

# Bundle Qt dependencies when linuxdeployqt is available.
if command -v linuxdeployqt >/dev/null; then
    linuxdeployqt "$OUT/usr/share/applications/openinfer-studio.desktop" \
        -appimage -qmldir=apps/desktop/qml -unsupported-allow-new-glibc
else
    echo "linuxdeployqt not found; AppDir assembled without bundled Qt libs."
    echo "Install Qt 6.5+ on the target system or run with linuxdeployqt."
fi
echo "Linux package staged at $OUT"
