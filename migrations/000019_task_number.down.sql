ALTER TABLE lg_tasks DROP CONSTRAINT IF EXISTS lg_tasks_workspace_number_unique;
ALTER TABLE lg_tasks DROP COLUMN IF EXISTS task_number;
