-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('ADMIN', 'USER')),
    avatar_path TEXT,
    date_of_birth DATE,
    theme TEXT NOT NULL DEFAULT 'light' CHECK (theme IN ('light', 'dark')),
    language TEXT NOT NULL DEFAULT 'ru' CHECK (language IN ('ru', 'en')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE storages (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('PERSONAL', 'GLOBAL')),
    visibility TEXT NOT NULL CHECK (visibility IN ('PRIVATE', 'PUBLIC_READ', 'PUBLIC_UPLOAD')),
    max_file_size BIGINT NOT NULL CHECK (max_file_size > 0),
    max_storage_size BIGINT NOT NULL CHECK (max_storage_size > 0),
    used_size BIGINT NOT NULL DEFAULT 0 CHECK (used_size >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE folders (
    id BIGSERIAL PRIMARY KEY,
    storage_id BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    parent_id BIGINT REFERENCES folders(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (length(trim(name)) > 0)
);

CREATE TABLE storage_accesses (
    id BIGSERIAL PRIMARY KEY,
    storage_id BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_level TEXT NOT NULL CHECK (access_level IN ('OWNER', 'MANAGER', 'UPLOADER', 'VIEWER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (storage_id, user_id)
);

CREATE TABLE folder_accesses (
    id BIGSERIAL PRIMARY KEY,
    folder_id BIGINT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_level TEXT NOT NULL CHECK (access_level IN ('MANAGER', 'UPLOADER', 'VIEWER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (folder_id, user_id)
);

CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    storage_id BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    folder_id BIGINT REFERENCES folders(id) ON DELETE SET NULL,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    original_name TEXT NOT NULL,
    stored_name TEXT NOT NULL UNIQUE,
    relative_path TEXT NOT NULL UNIQUE,
    mime_type TEXT NOT NULL,
    size BIGINT NOT NULL CHECK (size >= 0),
    checksum TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE storage_type_rules (
    id BIGSERIAL PRIMARY KEY,
    storage_id BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL CHECK (rule_type IN ('ALLOW', 'DENY')),
    pattern TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (storage_id, rule_type, pattern),
    CHECK (length(trim(pattern)) > 0)
);

CREATE TABLE share_links (
    id BIGSERIAL PRIMARY KEY,
    file_id BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    token TEXT,
    token_hash TEXT NOT NULL UNIQUE,
    access_type TEXT NOT NULL CHECK (access_type IN ('READ', 'WRITE')),
    expires_at TIMESTAMPTZ,
    use_count INTEGER NOT NULL DEFAULT 0 CHECK (use_count >= 0),
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at TIMESTAMPTZ
);

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id BIGINT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_storage_accesses_user_id ON storage_accesses(user_id);
CREATE INDEX idx_storage_accesses_storage_id ON storage_accesses(storage_id);

CREATE INDEX idx_folder_accesses_user_id ON folder_accesses(user_id);
CREATE INDEX idx_folder_accesses_folder_id ON folder_accesses(folder_id);

CREATE UNIQUE INDEX idx_folders_unique_root_name
    ON folders (storage_id, lower(name))
    WHERE parent_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX idx_folders_unique_child_name
    ON folders (storage_id, parent_id, lower(name))
    WHERE parent_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX idx_folders_storage_parent
    ON folders (storage_id, parent_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_files_storage_id ON files(storage_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_files_storage_folder ON files(storage_id, folder_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_files_folder_created ON files(storage_id, folder_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_files_folder_name ON files(storage_id, folder_id, lower(original_name), id) WHERE deleted_at IS NULL;
CREATE INDEX idx_files_original_name_trgm ON files USING gin (lower(original_name) gin_trgm_ops) WHERE deleted_at IS NULL;

CREATE INDEX idx_storage_type_rules_storage_id ON storage_type_rules(storage_id);

CREATE INDEX idx_share_links_file_id ON share_links(file_id);
CREATE INDEX idx_share_links_active_token_hash ON share_links(token_hash) WHERE is_active = true;
CREATE UNIQUE INDEX idx_share_links_token ON share_links(token) WHERE token IS NOT NULL;

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_active_hash ON refresh_tokens(token_hash) WHERE revoked_at IS NULL;

CREATE INDEX idx_audit_logs_user_id_created_at ON audit_logs(user_id, created_at DESC);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS share_links;
DROP TABLE IF EXISTS storage_type_rules;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS folder_accesses;
DROP TABLE IF EXISTS storage_accesses;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS storages;
DROP TABLE IF EXISTS users;
