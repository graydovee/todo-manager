-- Add custom_prompt column to summaries table
ALTER TABLE summaries ADD COLUMN custom_prompt TEXT;

-- Create followup_messages table
CREATE TABLE IF NOT EXISTS followup_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    summary_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (summary_id) REFERENCES summaries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_followup_messages_summary_id ON followup_messages(summary_id);

-- Create followup_message_versions table
CREATE TABLE IF NOT EXISTS followup_message_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    followup_message_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    version_number INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (followup_message_id) REFERENCES followup_messages(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_fmv_followup_message_id ON followup_message_versions(followup_message_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_fmv_message_version ON followup_message_versions(followup_message_id, version_number);
