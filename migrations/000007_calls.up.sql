ALTER TABLE lg_semantic_nodes ADD COLUMN IF NOT EXISTS calls TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX ON lg_semantic_nodes USING gin(calls);
