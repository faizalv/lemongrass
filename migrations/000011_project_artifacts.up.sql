CREATE TABLE lg_project_artifacts (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id BIGINT      NOT NULL REFERENCES lg_projects(id),
    type       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    content    TEXT        NOT NULL DEFAULT '',
    version    INT         NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON lg_project_artifacts (project_id, created_at DESC);
