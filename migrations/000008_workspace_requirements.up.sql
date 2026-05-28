CREATE TABLE lg_workspace_requirements (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES lg_workspaces(id) ON DELETE CASCADE,
  type         TEXT NOT NULL,
  text_content TEXT,
  file_path    TEXT,
  file_name    TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON lg_workspace_requirements (workspace_id);

ALTER TABLE lg_workspaces DROP COLUMN requirement_text;
ALTER TABLE lg_workspaces DROP COLUMN requirement_file;
ALTER TABLE lg_workspaces DROP COLUMN requirement_type;
