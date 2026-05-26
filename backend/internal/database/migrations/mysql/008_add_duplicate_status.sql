-- Add 'duplicate' to the status ENUM for todos table
ALTER TABLE todos MODIFY COLUMN status ENUM('open', 'in_progress', 'completed', 'duplicate') NOT NULL DEFAULT 'open';
