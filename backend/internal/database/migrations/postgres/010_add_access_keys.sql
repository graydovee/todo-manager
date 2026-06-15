CREATE TABLE IF NOT EXISTS access_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    key_prefix VARCHAR(32) NOT NULL,
    key_salt VARCHAR(128) NOT NULL,
    key_hash VARCHAR(256) NOT NULL,
    authorized_apis_json TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_access_keys_key_prefix ON access_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_access_keys_user_id ON access_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_access_keys_expires_at ON access_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_access_keys_last_used_at ON access_keys(last_used_at);
