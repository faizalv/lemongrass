ALTER TABLE lg_tasks
  DROP COLUMN IF EXISTS execution_status,
  DROP COLUMN IF EXISTS execution_notes,
  DROP COLUMN IF EXISTS execution_diff,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS finished_at,
  DROP COLUMN IF EXISTS rejection_reason;
