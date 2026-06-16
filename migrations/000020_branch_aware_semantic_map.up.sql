ALTER TABLE lg_semantic_nodes
  ADD COLUMN branches    TEXT[]    NOT NULL DEFAULT '{}',
  ADD COLUMN orphaned_at TIMESTAMPTZ;

UPDATE lg_semantic_nodes n
SET    branches = ARRAY[p.branch]
FROM   lg_projects p
WHERE  n.project_id = p.id;

DELETE FROM lg_semantic_nodes WHERE content_hash IS NULL;

ALTER TABLE lg_semantic_nodes
  DROP CONSTRAINT lg_semantic_nodes_project_id_file_path_symbol_kind_key;

ALTER TABLE lg_semantic_nodes
  ADD CONSTRAINT lg_semantic_nodes_unique_impl
  UNIQUE (project_id, file_path, symbol, kind, content_hash);

CREATE INDEX lg_semantic_nodes_branches_idx ON lg_semantic_nodes USING GIN (branches);

ALTER TABLE lg_projects ADD COLUMN last_synced_branch TEXT;
