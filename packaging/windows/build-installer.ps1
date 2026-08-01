# Package OpenInfer Studio as a Windows x64 installer (.exe) plus a portable zip.
# Requires: Go, CMake, Qt 6 (windeployqt), MSVC, Inno Setup 6 (ISCC.exe).
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..\..")

$Version = (Get-Content -Raw internal/version/VERSION).Trim()
$Payload = "dist\windows\payload"
$OutDir = "dist\windows"
Remove-Item -Recurse -Force dist\windows -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $Payload | Out-Null
New-Item -ItemType Directory -Force $OutDir | Out-Null

$Commit = "dev"
try { $Commit = (git rev-parse --short HEAD).Trim() } catch {}
$Date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mmZ")
$LdFlags = "-s -w -X github.com/openinfer/openinfer-studio/internal/version.Commit=$Commit -X github.com/openinfer/openinfer-studio/internal/version.Date=$Date"

Write-Host "==> building openinfer-core $Version"
go build -trimpath -ldflags $LdFlags -o "$Payload\openinfer-core.exe" ./apps/core

Write-Host "==> building openinfer-studio"
cmake -B build -S apps/desktop -DCMAKE_BUILD_TYPE=Release -G Ninja
cmake --build build --config Release
$studio = @(
    "build\openinfer-studio.exe",
    "build\Release\openinfer-studio.exe"
) | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $studio) { throw "openinfer-studio.exe not found under build/" }
Copy-Item $studio "$Payload\openinfer-studio.exe"

Write-Host "==> windeployqt"
windeployqt --release --qmldir apps/desktop/qml "$Payload\openinfer-studio.exe"

$Zip = "$OutDir\OpenInferStudio-$Version-windows-x86_64.zip"
if (Test-Path $Zip) { Remove-Item $Zip }
Compress-Archive -Path "$Payload\*" -DestinationPath $Zip
Write-Host "Portable zip: $Zip"

function Find-ISCC {
    $candidates = @(
        "${env:LOCALAPPDATA}\Programs\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles}\Inno Setup 6\ISCC.exe",
        "ISCC.exe"
    )
    foreach ($c in $candidates) {
        if (Get-Command $c -ErrorAction SilentlyContinue) { return (Get-Command $c).Source }
        if (Test-Path $c) { return $c }
    }
    return $null
}

$iscc = Find-ISCC
if (-not $iscc) {
    Write-Warning "Inno Setup (ISCC.exe) not found; skipping .exe installer. Portable zip is ready."
    exit 0
}

Write-Host "==> Inno Setup ($iscc)"
# Inno resolves relative Source/OutputDir paths against the .iss file
# directory, not the process cwd — always pass absolute paths.
$PayloadAbs = (Resolve-Path $Payload).Path
$OutDirAbs = (Resolve-Path $OutDir).Path
$IssAbs = (Resolve-Path "packaging\windows\openinfer.iss").Path
Write-Host "    payload=$PayloadAbs"
Write-Host "    output=$OutDirAbs"
& $iscc `
    "/DMyAppVersion=$Version" `
    "/DMyAppDir=$PayloadAbs" `
    "/DMyOutputDir=$OutDirAbs" `
    $IssAbs
if ($LASTEXITCODE -ne 0) { throw "ISCC failed with $LASTEXITCODE" }

$Setup = Join-Path $OutDirAbs "OpenInferStudio-$Version-windows-x86_64-setup.exe"
if (-not (Test-Path $Setup)) { throw "installer missing: $Setup" }
Write-Host "Installer: $Setup"
