CREATE TABLE lg_codebase_interim (
  id          BIGSERIAL PRIMARY KEY,
  session_id  TEXT NOT NULL,
  file_path   TEXT NOT NULL,
  chunk_index INT NOT NULL,
  content     TEXT NOT NULL,
  embedding   vector(768),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON lg_codebase_interim(session_id);
