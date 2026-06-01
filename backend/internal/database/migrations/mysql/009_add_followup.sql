-- Add custom_prompt column to summaries table
ALTER TABLE summaries ADD COLUMN custom_prompt TEXT;

-- Create followup_messages table
CREATE TABLE IF NOT EXISTS followup_messages (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    summary_id BIGINT UNSIGNED NOT NULL,
    question TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (summary_id) REFERENCES summaries(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_followup_messages_summary_id ON followup_messages(summary_id);

-- Create followup_message_versions table
CREATE TABLE IF NOT EXISTS followup_message_versions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    followup_message_id BIGINT UNSIGNED NOT NULL,
    content TEXT NOT NULL,
    version_number INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (followup_message_id) REFERENCES followup_messages(id) ON DELETE CASCADE,
    UNIQUE KEY idx_fmv_message_version (followup_message_id, version_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_fmv_followup_message_id ON followup_message_versions(followup_message_id);
