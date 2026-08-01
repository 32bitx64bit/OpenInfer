# OpenInfer Studio

Desktop app for finding, downloading, configuring, and running GGUF models
locally with llama.cpp. Search Hugging Face, manage llama.cpp builds, load
models with explicit engine settings, chat with streaming, and serve loaded
models through a local OpenAI-compatible API.

**Status:** early development. Discover → download → load → chat → serve works
end to end. See *Known limitations* below.

## Platforms

| Platform | Status | Notes |
|---|---|---|
| Linux x86_64 / arm64 | primary | AppImage / portable packaging |
| Windows x86_64 | supported | portable zip via windeployqt |
| macOS arm64 | supported | .app via macdeployqt; Metal preferred |
| macOS x86_64 | supported | CPU builds |

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
- Attachments (images/audio/PDF) are not wired; chat sends text only.
- Windows hardware detection is thinner than Linux.
- No dedicated first-run onboarding wizard yet.
- No software license selected yet.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md).
