ALTER TABLE lg_tasks ADD COLUMN task_number INTEGER NOT NULL DEFAULT 0;

UPDATE lg_tasks t
SET task_number = sub.rn
FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY workspace_id ORDER BY created_at ASC) AS rn
    FROM lg_tasks
) sub
WHERE t.id = sub.id;

ALTER TABLE lg_tasks ALTER COLUMN task_number DROP DEFAULT;
ALTER TABLE lg_tasks ADD CONSTRAINT lg_tasks_workspace_number_unique UNIQUE (workspace_id, task_number);
