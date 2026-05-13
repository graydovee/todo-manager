ALTER TABLE todos ADD COLUMN status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'completed'));

UPDATE todos SET status = 'completed' WHERE completed = 1;
