-- OpenInfer Studio initial schema. All timestamps are UTC RFC3339 strings.

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_directories (
    id         TEXT PRIMARY KEY,
    path       TEXT NOT NULL UNIQUE,
    managed    INTEGER NOT NULL DEFAULT 0, -- 1 = application-managed model dir
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS models (
    id             TEXT PRIMARY KEY,
    alias          TEXT NOT NULL DEFAULT '',
    favorite       INTEGER NOT NULL DEFAULT 0,
    notes          TEXT NOT NULL DEFAULT '',
    primary_path   TEXT NOT NULL,        -- main .gguf file (first shard for splits)
    projector_path TEXT NOT NULL DEFAULT '',
    size_bytes     INTEGER NOT NULL DEFAULT 0,
    quantization   TEXT NOT NULL DEFAULT '',
    architecture   TEXT NOT NULL DEFAULT '',
    parameters     INTEGER NOT NULL DEFAULT 0,
    context_length INTEGER NOT NULL DEFAULT 0,
    metadata_json  TEXT NOT NULL DEFAULT '{}',
    source_repo    TEXT NOT NULL DEFAULT '',
    pinned_runtime TEXT NOT NULL DEFAULT '', -- runtimes.id or ''
    pinned_backend TEXT NOT NULL DEFAULT '',
    last_loaded_at TEXT NOT NULL DEFAULT '',
    last_runtime   TEXT NOT NULL DEFAULT '',
    last_result    TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_models_favorite ON models(favorite);

CREATE TABLE IF NOT EXISTS model_files (
    id         TEXT PRIMARY KEY,
    model_id   TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'shard', -- primary|shard|projector
    size_bytes INTEGER NOT NULL DEFAULT 0,
    sha256     TEXT NOT NULL DEFAULT '',
    UNIQUE(model_id, path)
);

CREATE TABLE IF NOT EXISTS model_sources (
    id         TEXT PRIMARY KEY,
    model_id   TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider   TEXT NOT NULL DEFAULT 'huggingface',
    repo       TEXT NOT NULL DEFAULT '',
    revision   TEXT NOT NULL DEFAULT '',
    file_path  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_presets (
    id         TEXT PRIMARY KEY,
    model_id   TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    settings   TEXT NOT NULL DEFAULT '{}', -- JSON object of load settings
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(model_id, name)
);

CREATE TABLE IF NOT EXISTS runtimes (
    id              TEXT PRIMARY KEY,
    source          TEXT NOT NULL DEFAULT 'official-release', -- official-release|custom-import
    release_id      TEXT NOT NULL DEFAULT '',
    build           TEXT NOT NULL DEFAULT '',
    commit_hash     TEXT NOT NULL DEFAULT '',
    platform        TEXT NOT NULL DEFAULT '',
    architecture    TEXT NOT NULL DEFAULT '',
    backend         TEXT NOT NULL DEFAULT '',
    download_url    TEXT NOT NULL DEFAULT '',
    archive_sha256  TEXT NOT NULL DEFAULT '',
    installed_at    TEXT NOT NULL,
    install_dir     TEXT NOT NULL,
    executable_path TEXT NOT NULL,
    version_output  TEXT NOT NULL DEFAULT '',
    help_output     TEXT NOT NULL DEFAULT '',
    preferred       INTEGER NOT NULL DEFAULT 0,
    healthy         INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS runtime_capabilities (
    runtime_id TEXT NOT NULL REFERENCES runtimes(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    supported  INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (runtime_id, capability)
);

CREATE TABLE IF NOT EXISTS runtime_installations (
    id         TEXT PRIMARY KEY,
    release_id TEXT NOT NULL DEFAULT '',
    backend    TEXT NOT NULL DEFAULT '',
    state      TEXT NOT NULL DEFAULT 'pending', -- pending|downloading|verifying|staged|installed|failed
    progress   REAL NOT NULL DEFAULT 0,
    error      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_runtime_history (
    id         TEXT PRIMARY KEY,
    model_id   TEXT NOT NULL,
    runtime_id TEXT NOT NULL DEFAULT '',
    result     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS downloads (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL DEFAULT 'model', -- model|runtime
    label        TEXT NOT NULL DEFAULT '',
    state        TEXT NOT NULL DEFAULT 'queued', -- queued|active|paused|complete|failed|canceled
    queue_pos    INTEGER NOT NULL DEFAULT 0,
    total_bytes  INTEGER NOT NULL DEFAULT 0,
    done_bytes   INTEGER NOT NULL DEFAULT 0,
    dest_dir     TEXT NOT NULL DEFAULT '',
    error        TEXT NOT NULL DEFAULT '',
    meta_json    TEXT NOT NULL DEFAULT '{}',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS download_files (
    id           TEXT PRIMARY KEY,
    download_id  TEXT NOT NULL REFERENCES downloads(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    dest_path    TEXT NOT NULL,
    partial_path TEXT NOT NULL DEFAULT '',
    total_bytes  INTEGER NOT NULL DEFAULT 0,
    done_bytes   INTEGER NOT NULL DEFAULT 0,
    sha256       TEXT NOT NULL DEFAULT '',
    state        TEXT NOT NULL DEFAULT 'queued',
    resumable    INTEGER NOT NULL DEFAULT -1, -- -1 unknown, 0 no, 1 yes
    UNIQUE(download_id, dest_path)
);

CREATE TABLE IF NOT EXISTS instances (
    id           TEXT PRIMARY KEY,
    model_id     TEXT NOT NULL,
    preset_id    TEXT NOT NULL DEFAULT '',
    runtime_id   TEXT NOT NULL DEFAULT '',
    pid          INTEGER NOT NULL DEFAULT 0,
    port         INTEGER NOT NULL DEFAULT 0,
    state        TEXT NOT NULL DEFAULT 'unloaded',
    args_json    TEXT NOT NULL DEFAULT '[]',
    started_at   TEXT NOT NULL DEFAULT '',
    stopped_at   TEXT NOT NULL DEFAULT '',
    exit_code    INTEGER NOT NULL DEFAULT 0,
    failure      TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT 'New chat',
    model_id    TEXT NOT NULL DEFAULT '',
    system      TEXT NOT NULL DEFAULT '',
    params_json TEXT NOT NULL DEFAULT '{}',
    archived    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id          TEXT PRIMARY KEY,
    conv_id     TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    parent_id   TEXT NOT NULL DEFAULT '', -- '' = conversation root
    role        TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    reasoning   TEXT NOT NULL DEFAULT '',
    stats_json  TEXT NOT NULL DEFAULT '{}',
    error       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_conv ON conversation_messages(conv_id);

CREATE TABLE IF NOT EXISTS attachments (
    id         TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES conversation_messages(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL DEFAULT 'text', -- image|audio|video|text|pdf
    path       TEXT NOT NULL DEFAULT '',
    mime       TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL DEFAULT '',
    size_bytes INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS generation_presets (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    params     TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS server_profiles (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL DEFAULT 'default',
    port       INTEGER NOT NULL DEFAULT 1235,
    bind       TEXT NOT NULL DEFAULT '127.0.0.1',
    allow_lan  INTEGER NOT NULL DEFAULT 0,
    cors       TEXT NOT NULL DEFAULT '',
    autostart  INTEGER NOT NULL DEFAULT 0,
    api_key    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS diagnostic_events (
    id         TEXT PRIMARY KEY,
    kind       TEXT NOT NULL DEFAULT '',
    severity   TEXT NOT NULL DEFAULT 'info',
    summary    TEXT NOT NULL DEFAULT '',
    detail     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
