package repository

import (
	"context"
	"time"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
	"github.com/jmoiron/sqlx"
)

type workspaceRecord struct {
	ID        string    `db:"id"`
	ProjectID int64     `db:"project_id"`
	Name      string    `db:"name"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func toEntity(r workspaceRecord) entity.Workspace {
	return entity.Workspace{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		Name:      r.Name,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

type WorkspaceRepository struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error) {
	var rec workspaceRecord
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO lg_workspaces (project_id, name)
		 VALUES ($1, $2)
		 RETURNING id, project_id, name, status, created_at, updated_at`,
		ws.ProjectID,
		ws.Name,
	).StructScan(&rec)
	if err != nil {
		return entity.Workspace{}, err
	}
	return toEntity(rec), nil
}

func (r *WorkspaceRepository) Get(ctx context.Context, id string) (entity.Workspace, error) {
	var rec workspaceRecord
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, project_id, name, status, created_at, updated_at
		 FROM lg_workspaces WHERE id = $1`,
		id,
	).StructScan(&rec)
	if err != nil {
		return entity.Workspace{}, err
	}
	return toEntity(rec), nil
}

func (r *WorkspaceRepository) ListByProject(ctx context.Context, projectID int64, includeDeleted bool) ([]entity.Workspace, error) {
	var recs []workspaceRecord
	query := `SELECT id, project_id, name, status, created_at, updated_at
	          FROM lg_workspaces WHERE project_id = $1`
	if !includeDeleted {
		query += ` AND status != 'deleted'`
	}
	query += ` ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &recs, query, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]entity.Workspace, len(recs))
	for i, rec := range recs {
		out[i] = toEntity(rec)
	}
	return out, nil
}

func (r *WorkspaceRepository) CountExecuting(ctx context.Context, projectID int64) (int, error) {
	var count int
	err := r.db.QueryRowxContext(ctx,
		`SELECT COUNT(*) FROM lg_workspaces WHERE project_id = $1 AND status = 'executing'`,
		projectID,
	).Scan(&count)
	return count, err
}

func (r *WorkspaceRepository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_workspaces SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	return err
}

func (r *WorkspaceRepository) GetProjectPath(ctx context.Context, projectID int64) (string, error) {
	var path string
	err := r.db.QueryRowContext(ctx,
		`SELECT path FROM lg_projects WHERE id = $1`, projectID,
	).Scan(&path)
	return path, err
}

func (r *WorkspaceRepository) DeleteByProject(ctx context.Context, projectID int64) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM lg_tasks WHERE workspace_id IN (SELECT id FROM lg_workspaces WHERE project_id = $1)`,
		projectID,
	); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_workspaces WHERE project_id = $1`, projectID)
	return err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
