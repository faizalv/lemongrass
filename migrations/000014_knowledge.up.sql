CREATE TABLE lg_knowledge (
    id         BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES lg_projects(id),
    key        TEXT NOT NULL,
    content    TEXT NOT NULL,
    embedding  vector(768),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key)
);
