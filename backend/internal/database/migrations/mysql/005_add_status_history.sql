-- Track status transitions for todos
CREATE TABLE IF NOT EXISTS todo_status_history (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    todo_id BIGINT UNSIGNED NOT NULL,
    old_status VARCHAR(20) NOT NULL,
    new_status VARCHAR(20) NOT NULL,
    changed_at DATETIME NOT NULL,
    FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE INDEX idx_status_history_todo_id ON todo_status_history(todo_id);
CREATE INDEX idx_status_history_changed_at ON todo_status_history(changed_at);
