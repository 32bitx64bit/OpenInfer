# Package OpenInfer Studio as a portable Windows zip.
# Run on Windows with Qt 6 + MSVC and windeployqt available.
$ErrorActionPreference = "Stop"
$Version = "0.1.0"
$Dist = "dist/OpenInferStudio-$Version-windows-x86_64"

Remove-Item -Recurse -Force $Dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $Dist | Out-Null

go build -trimpath -ldflags "-s -w" -o "$Dist/openinfer-core.exe" ./apps/core

cmake -B build -S apps/desktop -DCMAKE_BUILD_TYPE=Release
cmake --build build --config Release
Copy-Item build/Release/openinfer-studio.exe $Dist/ -ErrorAction SilentlyContinue
Copy-Item build/openinfer-studio.exe $Dist/ -ErrorAction SilentlyContinue

# Deploy Qt runtime next to the executable.
windeployqt --qmldir apps/desktop/qml "$Dist/openinfer-studio.exe"

Compress-Archive -Path "$Dist/*" -DestinationPath "dist/OpenInferStudio-$Version-windows-x86_64.zip"
Write-Host "Packaged: dist/OpenInferStudio-$Version-windows-x86_64.zip"
