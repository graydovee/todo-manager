-- Add todo_ids column to store selected todo IDs for analysis
ALTER TABLE summaries ADD COLUMN IF NOT EXISTS todo_ids TEXT;
