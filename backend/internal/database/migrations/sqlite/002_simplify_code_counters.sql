-- Recreate code_counters table without category column.
-- SQLite doesn't support DROP COLUMN directly, so we recreate the table.
CREATE TABLE code_counters_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    last_code INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT idx_user_id UNIQUE (user_id)
);

INSERT INTO code_counters_new (id, user_id, last_code)
SELECT id, user_id, last_code FROM code_counters
GROUP BY user_id
HAVING id = MAX(id);

DROP TABLE code_counters;
ALTER TABLE code_counters_new RENAME TO code_counters;
