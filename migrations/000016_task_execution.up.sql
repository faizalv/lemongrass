ALTER TABLE lg_tasks
  ADD COLUMN execution_status TEXT NOT NULL DEFAULT '',
  ADD COLUMN execution_notes TEXT NOT NULL DEFAULT '',
  ADD COLUMN execution_diff JSONB,
  ADD COLUMN started_at TIMESTAMPTZ,
  ADD COLUMN finished_at TIMESTAMPTZ,
  ADD COLUMN rejection_reason TEXT NOT NULL DEFAULT '';
