-- Track status transitions for todos
CREATE TABLE IF NOT EXISTS todo_status_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    todo_id INTEGER NOT NULL,
    old_status TEXT NOT NULL,
    new_status TEXT NOT NULL,
    changed_at DATETIME NOT NULL,
    FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_status_history_todo_id ON todo_status_history(todo_id);
CREATE INDEX IF NOT EXISTS idx_status_history_changed_at ON todo_status_history(changed_at);
