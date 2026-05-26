-- Add 'duplicate' to the status CHECK constraint for todos table
ALTER TABLE todos DROP CONSTRAINT IF EXISTS todos_status_check;
ALTER TABLE todos ADD CONSTRAINT todos_status_check CHECK (status IN ('open', 'in_progress', 'completed', 'duplicate'));
