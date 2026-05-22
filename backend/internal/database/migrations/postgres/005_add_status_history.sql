-- Track status transitions for todos
CREATE TABLE IF NOT EXISTS todo_status_history (
    id BIGSERIAL PRIMARY KEY,
    todo_id BIGINT NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
    old_status VARCHAR(20) NOT NULL,
    new_status VARCHAR(20) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_status_history_todo_id ON todo_status_history(todo_id);
CREATE INDEX IF NOT EXISTS idx_status_history_changed_at ON todo_status_history(changed_at);
