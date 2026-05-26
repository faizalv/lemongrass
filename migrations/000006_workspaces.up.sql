CREATE TABLE lg_workspaces (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id       BIGINT NOT NULL REFERENCES lg_projects(id),
  name             TEXT NOT NULL,
  status           TEXT NOT NULL DEFAULT 'idle',
  requirement_text TEXT,
  requirement_file TEXT,
  requirement_type TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON lg_workspaces (project_id);
CREATE INDEX ON lg_workspaces (project_id, status);

CREATE TABLE lg_tasks (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES lg_workspaces(id),
  title        TEXT NOT NULL,
  impl         JSONB,
  status       TEXT NOT NULL DEFAULT 'pending',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  approved_at  TIMESTAMPTZ
);

ALTER TABLE lg_projects ADD COLUMN branch TEXT NOT NULL DEFAULT 'main';
