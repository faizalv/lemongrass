DROP INDEX IF EXISTS lg_knowledge_labels_project_id_idx;
DROP INDEX IF EXISTS lg_knowledge_project_id_labels_idx;
DROP TABLE IF EXISTS lg_knowledge_labels;
ALTER TABLE lg_knowledge DROP COLUMN IF EXISTS labels;
