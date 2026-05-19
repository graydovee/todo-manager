ALTER TABLE code_counters DROP CONSTRAINT idx_user_category;
ALTER TABLE code_counters DROP COLUMN category;
ALTER TABLE code_counters ADD CONSTRAINT idx_user_id UNIQUE (user_id);
