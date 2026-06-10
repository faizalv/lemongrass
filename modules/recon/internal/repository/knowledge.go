package repository

import (
	"context"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/lib/pq"
)

func (r *ReconRepository) SaveKnowledge(ctx context.Context, projectID int64, key, content string, embedding []float32, labels []string) error {
	if labels == nil {
		labels = []string{}
	}
	if len(embedding) > 0 {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO lg_knowledge (project_id, key, content, embedding, labels, updated_at)
			 VALUES ($1, $2, $3, $4::vector, $5, NOW())
			 ON CONFLICT (project_id, key) DO UPDATE
			 SET content = EXCLUDED.content, embedding = EXCLUDED.embedding, labels = EXCLUDED.labels, updated_at = NOW()`,
			projectID, key, content, formatVector(embedding), pq.Array(labels),
		)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO lg_knowledge (project_id, key, content, embedding, labels, updated_at)
		 VALUES ($1, $2, $3, NULL, $4, NOW())
		 ON CONFLICT (project_id, key) DO UPDATE
		 SET content = EXCLUDED.content, embedding = NULL, labels = EXCLUDED.labels, updated_at = NOW()`,
		projectID, key, content, pq.Array(labels),
	)
	return err
}

func (r *ReconRepository) ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error) {
	var content string
	err := r.db.QueryRowContext(ctx,
		`SELECT content FROM lg_knowledge WHERE project_id = $1 AND key = $2`,
		projectID, key,
	).Scan(&content)
	return content, err
}

func (r *ReconRepository) SearchKnowledge(ctx context.Context, projectID int64, embedding []float32, limit int) ([]entity.KnowledgeEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, content, updated_at FROM lg_knowledge
		 WHERE project_id = $1 AND embedding IS NOT NULL
		 ORDER BY embedding <=> $2::vector
		 LIMIT $3`,
		projectID, formatVector(embedding), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.KnowledgeEntry
	for rows.Next() {
		var e entity.KnowledgeEntry
		if err := rows.Scan(&e.Key, &e.Content, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *ReconRepository) ListKnowledge(ctx context.Context, projectID int64) ([]entity.KnowledgeEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, content, updated_at FROM lg_knowledge
		 WHERE project_id = $1
		 ORDER BY updated_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.KnowledgeEntry
	for rows.Next() {
		var e entity.KnowledgeEntry
		if err := rows.Scan(&e.Key, &e.Content, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *ReconRepository) DeleteKnowledge(ctx context.Context, projectID int64, key string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM lg_knowledge WHERE project_id = $1 AND key = $2`,
		projectID, key,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *ReconRepository) FindSimilarKnowledge(ctx context.Context, projectID int64, excludeKey string, embedding []float32) ([]string, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT key FROM lg_knowledge
		 WHERE project_id = $1
		   AND key != $2
		   AND embedding IS NOT NULL
		   AND embedding <=> $3::vector < 0.20
		 ORDER BY embedding <=> $3::vector
		 LIMIT 5`,
		projectID, excludeKey, formatVector(embedding),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *ReconRepository) DeleteKnowledgeByProject(ctx context.Context, projectID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_knowledge WHERE project_id = $1`, projectID)
	return err
}

func (r *ReconRepository) DeleteLabelsByProject(ctx context.Context, projectID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_knowledge_labels WHERE project_id = $1`, projectID)
	return err
}

func (r *ReconRepository) UpsertLabel(ctx context.Context, projectID int64, label string, embedding []float32) error {
	if len(embedding) == 0 {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO lg_knowledge_labels (project_id, label, embedding, updated_at)
			 VALUES ($1, $2, NULL, NOW())
			 ON CONFLICT (project_id, label) DO UPDATE
			 SET updated_at = NOW()`,
			projectID, label,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO lg_knowledge_labels (project_id, label, embedding, updated_at)
		 VALUES ($1, $2, $3::vector, NOW())
		 ON CONFLICT (project_id, label) DO UPDATE
		 SET embedding = EXCLUDED.embedding, updated_at = NOW()`,
		projectID, label, formatVector(embedding),
	)
	return err
}

func (r *ReconRepository) FindSimilarLabels(ctx context.Context, projectID int64, label string, embedding []float32) ([]string, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT label FROM lg_knowledge_labels
		 WHERE project_id = $1
		   AND label != $2
		   AND embedding IS NOT NULL
		   AND embedding <=> $3::vector < 0.20
		 ORDER BY embedding <=> $3::vector
		 LIMIT 3`,
		projectID, label, formatVector(embedding),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

func (r *ReconRepository) ListAllLabels(ctx context.Context, projectID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT label FROM lg_knowledge_labels
		 WHERE project_id = $1
		 ORDER BY label`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

func (r *ReconRepository) SearchLabels(ctx context.Context, projectID int64, embedding []float32) ([]string, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT label FROM lg_knowledge_labels
		 WHERE project_id = $1
		   AND embedding IS NOT NULL
		 ORDER BY embedding <=> $2::vector
		 LIMIT 5`,
		projectID, formatVector(embedding),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

func (r *ReconRepository) SearchKnowledgeByLabel(ctx context.Context, projectID int64, label string, embedding []float32, limit int) ([]entity.KnowledgeEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, content, updated_at FROM lg_knowledge
		 WHERE project_id = $1
		   AND $2 = ANY(labels)
		   AND embedding IS NOT NULL
		 ORDER BY embedding <=> $3::vector
		 LIMIT $4`,
		projectID, label, formatVector(embedding), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.KnowledgeEntry
	for rows.Next() {
		var e entity.KnowledgeEntry
		if err := rows.Scan(&e.Key, &e.Content, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
