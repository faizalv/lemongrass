CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS lg_semantic_nodes (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id   BIGINT NOT NULL,

  file_path    TEXT NOT NULL,
  line_start   INT  NOT NULL,
  line_end     INT  NOT NULL,

  package      TEXT NOT NULL,
  symbol       TEXT NOT NULL,
  kind         TEXT NOT NULL,
  language     TEXT NOT NULL,
  receiver     TEXT,
  signature    TEXT,
  exported     BOOL NOT NULL DEFAULT true,
  depends_on   TEXT[] NOT NULL DEFAULT '{}',

  status       TEXT NOT NULL DEFAULT 'unexplored',
  description  TEXT,
  return_type  TEXT,
  embedding    vector(768),
  explored_at  TIMESTAMPTZ,

  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (project_id, file_path, symbol, kind)
);

CREATE INDEX ON lg_semantic_nodes (project_id, status);
CREATE INDEX ON lg_semantic_nodes (project_id, file_path);
CREATE INDEX ON lg_semantic_nodes (project_id, language);
CREATE INDEX ON lg_semantic_nodes USING ivfflat (embedding vector_cosine_ops) WHERE embedding IS NOT NULL;
CREATE INDEX ON lg_semantic_nodes USING gin(to_tsvector('english', coalesce(description, '')));
