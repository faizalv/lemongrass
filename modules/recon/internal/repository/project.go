package repository

import (
	"context"
	"database/sql"
)

func (r *ReconRepository) ProjectDir(ctx context.Context, projectID int64) (string, error) {
	var path string
	err := r.db.QueryRowContext(ctx, `SELECT path FROM lg_projects WHERE id = $1`, projectID).Scan(&path)
	return path, err
}

func (r *ReconRepository) DeleteByProject(ctx context.Context, projectID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM lg_semantic_nodes WHERE project_id = $1`, projectID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_file_hashes WHERE project_id = $1`, projectID)
	return err
}

func (r *ReconRepository) GetSyncInterval(ctx context.Context, projectID int64) (string, error) {
	var interval string
	err := r.db.QueryRowContext(ctx,
		`SELECT sync_interval FROM lg_projects WHERE id = $1`, projectID).Scan(&interval)
	return interval, err
}

func (r *ReconRepository) UpdateSyncInterval(ctx context.Context, projectID int64, interval string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_projects SET sync_interval = $1 WHERE id = $2`, interval, projectID)
	return err
}

func (r *ReconRepository) GetLastSyncedCommit(ctx context.Context, projectID int64) (string, error) {
	var commit sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT last_synced_commit FROM lg_projects WHERE id = $1`, projectID).Scan(&commit)
	if err != nil {
		return "", err
	}
	return commit.String, nil
}

func (r *ReconRepository) SetLastSyncedCommit(ctx context.Context, projectID int64, commit string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_projects SET last_synced_commit = $1 WHERE id = $2`, commit, projectID)
	return err
}

func (r *ReconRepository) GetLastSyncedBranch(ctx context.Context, projectID int64) (string, error) {
	var branch sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT last_synced_branch FROM lg_projects WHERE id = $1`, projectID).Scan(&branch)
	if err != nil {
		return "", err
	}
	return branch.String, nil
}

func (r *ReconRepository) SetLastSyncedBranch(ctx context.Context, projectID int64, branch string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_projects SET last_synced_branch = $1 WHERE id = $2`, branch, projectID)
	return err
}
