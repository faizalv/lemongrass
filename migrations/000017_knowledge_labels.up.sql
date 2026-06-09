ALTER TABLE lg_knowledge ADD COLUMN labels TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE lg_knowledge_labels (
  id         BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES lg_projects(id),
  label      TEXT NOT NULL,
  embedding  vector(768),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(project_id, label)
);

CREATE INDEX ON lg_knowledge(project_id, labels) USING GIN;
CREATE INDEX ON lg_knowledge_labels(project_id);
