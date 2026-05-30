package repository

import (
	"context"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/lib/pq"
)

func (r *ReconRepository) GetFileHashes(ctx context.Context, projectID int64) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT file_path, hash FROM lg_file_hashes WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		out[path] = hash
	}
	return out, rows.Err()
}

func (r *ReconRepository) UpsertFileHashes(ctx context.Context, projectID int64, hashes []entity.FileHash) error {
	for _, h := range hashes {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO lg_file_hashes (project_id, file_path, hash, updated_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (project_id, file_path) DO UPDATE SET hash = EXCLUDED.hash, updated_at = NOW()`,
			projectID, h.Path, h.Hash,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconRepository) DeleteFileHashes(ctx context.Context, projectID int64, paths []string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM lg_file_hashes WHERE project_id = $1 AND file_path = ANY($2)`,
		projectID, pq.StringArray(paths),
	)
	return err
}
