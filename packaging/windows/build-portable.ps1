# Package OpenInfer Studio as a portable Windows zip (no installer).
# Prefer packaging/windows/build-installer.ps1 for the .exe setup.
$ErrorActionPreference = "Stop"
& "$PSScriptRoot\build-installer.ps1"
