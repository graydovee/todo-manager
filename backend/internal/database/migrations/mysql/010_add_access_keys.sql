CREATE TABLE IF NOT EXISTS access_keys (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(64) NOT NULL,
    key_prefix VARCHAR(32) NOT NULL,
    key_salt VARCHAR(128) NOT NULL,
    key_hash VARCHAR(256) NOT NULL,
    authorized_apis_json TEXT NOT NULL,
    expires_at DATETIME NULL,
    last_used_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY idx_access_keys_key_prefix (key_prefix),
    KEY idx_access_keys_user_id (user_id),
    KEY idx_access_keys_expires_at (expires_at),
    KEY idx_access_keys_last_used_at (last_used_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
