CREATE TABLE IF NOT EXISTS lg_meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

INSERT INTO lg_meta (key, value)
VALUES ('schema_version', '1')
ON CONFLICT (key) DO NOTHING;
