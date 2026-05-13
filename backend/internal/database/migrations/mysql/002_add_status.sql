ALTER TABLE todos ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'open';
ALTER TABLE todos ADD CONSTRAINT chk_todos_status CHECK (status IN ('open', 'in_progress', 'completed'));

UPDATE todos SET status = 'completed' WHERE completed = 1;
