DROP TABLE IF EXISTS lg_workspace_requirements;

ALTER TABLE lg_workspaces ADD COLUMN requirement_text TEXT;
ALTER TABLE lg_workspaces ADD COLUMN requirement_file TEXT;
ALTER TABLE lg_workspaces ADD COLUMN requirement_type TEXT;
