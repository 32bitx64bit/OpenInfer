# OpenInfer Studio

Desktop app for finding, downloading, configuring, and running GGUF models
locally with llama.cpp. Search Hugging Face, manage llama.cpp builds, load
models with explicit engine settings, chat with streaming, and serve loaded
models through a local OpenAI-compatible API.

**Status:** 1.1.0. Discover → download → load → chat → serve works end to end.
See *Known limitations* below.

## Platforms

| Platform | Status | Notes |
|---|---|---|
| Linux x86_64 | primary | AppImage via `packaging/linux/build-appimage.sh x86_64` |
| Linux aarch64 | supported | AppImage via `packaging/linux/build-appimage.sh aarch64` (native arm64 host) |
| Windows x86_64 | supported | `.exe` installer + portable zip |
| macOS arm64 | supported | `.dmg` via `packaging/macos/build-bundle.sh arm64` |
| macOS x86_64 | supported | `.dmg` via `packaging/macos/build-bundle.sh x86_64` |

## Architecture

```
openinfer-studio (Qt 6 / QML)
    │  REST + WebSocket, loopback only
    ▼
openinfer-core (Go)
    │  one managed process per loaded model
    ▼
llama-server (official llama.cpp builds, pinable per model)
```

- **C++ bootstrap** launches the backend and loads QML — no app logic.
- **Go backend** owns Hugging Face browsing, downloads, runtimes, process
  supervision, chat, and the OpenAI-compatible proxy.
- **QML** is the UI; it talks to the backend over an authenticated local API.

Control API reference: [`docs/api.md`](docs/api.md).

## Build

Requirements: Go ≥ 1.26, CMake ≥ 3.24, Qt ≥ 6.5 (Quick, QuickControls2,
WebSockets, Widgets), C++17.

```bash
./scripts/build.sh          # debug → build/
./scripts/build.sh release  # optimized
./scripts/test.sh           # Go tests + backend self-test
./build/openinfer-studio    # run
```

Backend alone (development):

```bash
go run ./apps/core --token dev-token --port 0 --data-dir /tmp/oi-dev
```

## Data directories

| Platform | Path |
|---|---|
| Linux | `~/.local/share/openinfer-studio` (config `~/.config/…`, cache `~/.cache/…`) |
| Windows | `%LOCALAPPDATA%\OpenInfer Studio` |
| macOS | `~/Library/Application Support/OpenInfer Studio` |

Contains `database/`, `runtimes/`, `models/`, `downloads/`, `cache/`, `logs/`,
`presets/`, `sessions/`, `temp/`. Nothing else is written without an explicit
user action.

## Privacy

Network use is limited to Hugging Face, llama.cpp release/runtime downloads,
and links you open. No telemetry, accounts, or cloud inference. Chats, prompts,
models, and hardware info stay local. Offline once models and runtimes are
installed.

The control API is loopback-only with a session token. The optional public
OpenAI-compatible server is a separate process with its own key; LAN bind is
opt-in. Inference processes bind loopback with per-process keys.

## Troubleshooting

- **Backend did not become ready** — run
  `openinfer-core --selftest --token t --data-dir /tmp/oi` and check
  `logs/application/core.log`.
- **Model fails to load** — the dialog shows the classified cause, generated
  command, and log tail. Retry with safe settings or CPU fallback from there.
- **Download stuck** — Downloads page shows resume state; partials under
  `downloads/partial/` resume automatically.
- **GPU unused** — Settings → Hardware for the detected backend, then install a
  matching runtime (Vulkan / CUDA / HIP / Metal).

## Known limitations

- Chat Markdown has no syntax highlighting yet.
- Image and PDF chat attachments are not wired yet. Experimental **audio**
  attachments can be enabled under Settings → Experimental → Audio models
  (mirrors llama.cpp’s experimental libmtmd audio input; quality may vary).
- Windows hardware detection is thinner than Linux.
- No dedicated first-run onboarding wizard yet.
- No software license selected yet.

## Releases

Version lives in `internal/version/VERSION`. Tag `vX.Y.Z` (matching that file)
to trigger `.github/workflows/release.yml`, which publishes:

- Linux: `OpenInferStudio-*-linux-x86_64.AppImage` and `OpenInferStudio-*-linux-aarch64.AppImage`
- Windows: `OpenInferStudio-*-windows-x86_64-setup.exe` (+ portable `.zip`)
- macOS: `OpenInferStudio-*-macos-arm64.dmg` and `OpenInferStudio-*-macos-x86_64.dmg`

Local packaging (on each OS):

```bash
./packaging/linux/build-appimage.sh x86_64
./packaging/linux/build-appimage.sh aarch64   # on an arm64 Linux host
./packaging/macos/build-bundle.sh arm64
./packaging/macos/build-bundle.sh x86_64
pwsh ./packaging/windows/build-installer.ps1
```
## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).
