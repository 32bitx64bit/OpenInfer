$ErrorActionPreference = "Stop"
$Version = (Get-Content -Raw internal/version/VERSION).Trim()
$Dist = "dist/OpenInferStudio-$Version-windows-x86_64"

Remove-Item -Recurse -Force $Dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $Dist | Out-Null

$Commit = "dev"
try { $Commit = (git rev-parse --short HEAD) } catch {}
$Date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mmZ")
$LdFlags = "-s -w -X github.com/openinfer/openinfer-studio/internal/version.Commit=$Commit -X github.com/openinfer/openinfer-studio/internal/version.Date=$Date"

go build -trimpath -ldflags $LdFlags -o "$Dist/openinfer-core.exe" ./apps/core

cmake -B build -S apps/desktop -DCMAKE_BUILD_TYPE=Release
cmake --build build --config Release
Copy-Item build/Release/openinfer-studio.exe $Dist/ -ErrorAction SilentlyContinue
Copy-Item build/openinfer-studio.exe $Dist/ -ErrorAction SilentlyContinue

# Deploy Qt runtime next to the executable.
windeployqt --qmldir apps/desktop/qml "$Dist/openinfer-studio.exe"

Compress-Archive -Path "$Dist/*" -DestinationPath "dist/OpenInferStudio-$Version-windows-x86_64.zip"
Write-Host "Packaged: dist/OpenInferStudio-$Version-windows-x86_64.zip"
