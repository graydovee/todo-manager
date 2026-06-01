-- Add custom_prompt column to summaries table
ALTER TABLE summaries ADD COLUMN custom_prompt TEXT;

-- Create followup_messages table
CREATE TABLE IF NOT EXISTS followup_messages (
    id BIGSERIAL PRIMARY KEY,
    summary_id BIGINT NOT NULL REFERENCES summaries(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_followup_messages_summary_id ON followup_messages(summary_id);

-- Create followup_message_versions table
CREATE TABLE IF NOT EXISTS followup_message_versions (
    id BIGSERIAL PRIMARY KEY,
    followup_message_id BIGINT NOT NULL REFERENCES followup_messages(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    version_number INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fmv_followup_message_id ON followup_message_versions(followup_message_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_fmv_message_version ON followup_message_versions(followup_message_id, version_number);
