-- Add retry scheduling and dead-letter observability for DB-backed workers.

ALTER TABLE event_inbox
  ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE task_items
  ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_event_inbox_status_next_attempt
  ON event_inbox (status, next_attempt_at, received_at);

CREATE INDEX IF NOT EXISTS idx_task_items_status_next_attempt
  ON task_items (status, next_attempt_at, updated_at);
