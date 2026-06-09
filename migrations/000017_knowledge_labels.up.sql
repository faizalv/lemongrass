ALTER TABLE lg_knowledge ADD COLUMN labels TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE lg_knowledge_labels (
  id         BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES lg_projects(id),
  label      TEXT NOT NULL,
  embedding  vector(768),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(project_id, label)
);

CREATE INDEX lg_knowledge_labels_gin_idx ON lg_knowledge USING GIN (labels);
CREATE INDEX lg_knowledge_labels_project_id_idx ON lg_knowledge_labels(project_id);
