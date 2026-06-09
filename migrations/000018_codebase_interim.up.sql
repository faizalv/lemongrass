CREATE TABLE lg_codebase_interim (
  id          BIGSERIAL PRIMARY KEY,
  session_id  TEXT NOT NULL,
  file_path   TEXT NOT NULL,
  chunk_index INT NOT NULL,
  content     TEXT NOT NULL,
  embedding   vector(768),
  line_start  INT NOT NULL DEFAULT 0,
  line_end    INT NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON lg_codebase_interim(session_id);
