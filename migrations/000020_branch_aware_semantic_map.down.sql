ALTER TABLE lg_projects DROP COLUMN IF EXISTS last_synced_branch;
DROP INDEX IF EXISTS lg_semantic_nodes_branches_idx;
ALTER TABLE lg_semantic_nodes DROP CONSTRAINT IF EXISTS lg_semantic_nodes_unique_impl;
ALTER TABLE lg_semantic_nodes DROP COLUMN IF EXISTS orphaned_at;
ALTER TABLE lg_semantic_nodes DROP COLUMN IF EXISTS branches;
ALTER TABLE lg_semantic_nodes ADD CONSTRAINT lg_semantic_nodes_project_id_file_path_symbol_kind_key UNIQUE (project_id, file_path, symbol, kind);
