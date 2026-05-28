package repository

import (
	"context"
	"encoding/json"
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

func (r *WorkspaceRepository) ListByProject(ctx context.Context, projectID int64) ([]entity.Workspace, error) {
	var recs []workspaceRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT id, project_id, name, status, created_at, updated_at
		 FROM lg_workspaces WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
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

// -- Requirements --

type requirementRecord struct {
	ID          string    `db:"id"`
	WorkspaceID string    `db:"workspace_id"`
	Type        string    `db:"type"`
	TextContent *string   `db:"text_content"`
	FilePath    *string   `db:"file_path"`
	FileName    *string   `db:"file_name"`
	CreatedAt   time.Time `db:"created_at"`
}

func toRequirementEntity(r requirementRecord) entity.WorkspaceRequirement {
	req := entity.WorkspaceRequirement{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Type:        r.Type,
		CreatedAt:   r.CreatedAt,
	}
	if r.TextContent != nil {
		req.TextContent = *r.TextContent
	}
	if r.FilePath != nil {
		req.FilePath = *r.FilePath
	}
	if r.FileName != nil {
		req.FileName = *r.FileName
	}
	return req
}

func (r *WorkspaceRepository) ListRequirements(ctx context.Context, workspaceID string) ([]entity.WorkspaceRequirement, error) {
	var recs []requirementRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT id, workspace_id, type, text_content, file_path, file_name, created_at
		 FROM lg_workspace_requirements WHERE workspace_id = $1 ORDER BY created_at ASC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	out := make([]entity.WorkspaceRequirement, len(recs))
	for i, rec := range recs {
		out[i] = toRequirementEntity(rec)
	}
	return out, nil
}

func (r *WorkspaceRepository) AddTextRequirement(ctx context.Context, workspaceID, text string) (entity.WorkspaceRequirement, error) {
	var rec requirementRecord
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO lg_workspace_requirements (workspace_id, type, text_content)
		 VALUES ($1, 'text', $2)
		 RETURNING id, workspace_id, type, text_content, file_path, file_name, created_at`,
		workspaceID, text,
	).StructScan(&rec)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	return toRequirementEntity(rec), nil
}

func (r *WorkspaceRepository) AddFileRequirement(ctx context.Context, workspaceID, reqType, filePath, fileName string) (entity.WorkspaceRequirement, error) {
	var rec requirementRecord
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO lg_workspace_requirements (workspace_id, type, file_path, file_name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, workspace_id, type, text_content, file_path, file_name, created_at`,
		workspaceID, reqType, filePath, fileName,
	).StructScan(&rec)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	return toRequirementEntity(rec), nil
}

func (r *WorkspaceRepository) DeleteRequirement(ctx context.Context, reqID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_workspace_requirements WHERE id = $1`, reqID)
	return err
}

func (r *WorkspaceRepository) CountRequirements(ctx context.Context, workspaceID string) (int, error) {
	var count int
	err := r.db.QueryRowxContext(ctx,
		`SELECT COUNT(*) FROM lg_workspace_requirements WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&count)
	return count, err
}

// -- Tasks --

type taskRecord struct {
	ID          string          `db:"id"`
	WorkspaceID string          `db:"workspace_id"`
	Title       string          `db:"title"`
	Impl        json.RawMessage `db:"impl"`
	Status      string          `db:"status"`
	CreatedAt   time.Time       `db:"created_at"`
	ApprovedAt  *time.Time      `db:"approved_at"`
}

func toTaskEntity(r taskRecord) entity.Task {
	return entity.Task{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Title:       r.Title,
		Impl:        r.Impl,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt,
		ApprovedAt:  r.ApprovedAt,
	}
}

func (r *WorkspaceRepository) CreateTasks(ctx context.Context, workspaceID string, tasks []entity.Task) ([]entity.Task, error) {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM lg_tasks WHERE workspace_id = $1`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	out := make([]entity.Task, 0, len(tasks))
	for _, t := range tasks {
		impl := t.Impl
		if impl == nil {
			impl = json.RawMessage("[]")
		}
		var rec taskRecord
		err := r.db.QueryRowxContext(ctx,
			`INSERT INTO lg_tasks (workspace_id, title, impl)
			 VALUES ($1, $2, $3)
			 RETURNING id, workspace_id, title, impl, status, created_at, approved_at`,
			workspaceID, t.Title, impl,
		).StructScan(&rec)
		if err != nil {
			return nil, err
		}
		out = append(out, toTaskEntity(rec))
	}
	return out, nil
}

func (r *WorkspaceRepository) GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	var recs []taskRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT id, workspace_id, title, impl, status, created_at, approved_at
		 FROM lg_tasks WHERE workspace_id = $1 ORDER BY created_at ASC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	out := make([]entity.Task, len(recs))
	for i, rec := range recs {
		out[i] = toTaskEntity(rec)
	}
	return out, nil
}

func (r *WorkspaceRepository) ApproveTasks(ctx context.Context, workspaceID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_tasks SET status = 'approved', approved_at = NOW()
		 WHERE workspace_id = $1 AND status = 'pending'`,
		workspaceID,
	)
	return err
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
