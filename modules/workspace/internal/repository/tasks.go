package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type taskRecord struct {
	ID                string          `db:"id"`
	WorkspaceID       string          `db:"workspace_id"`
	Title             string          `db:"title"`
	Reason            string          `db:"reason"`
	Impl              json.RawMessage `db:"impl"`
	Status            string          `db:"status"`
	AmendmentFeedback string          `db:"amendment_feedback"`
	CreatedAt         time.Time       `db:"created_at"`
	ApprovedAt        *time.Time      `db:"approved_at"`
}

func toTaskEntity(r taskRecord) entity.Task {
	return entity.Task{
		ID:                r.ID,
		WorkspaceID:       r.WorkspaceID,
		Title:             r.Title,
		Reason:            r.Reason,
		Impl:              r.Impl,
		Status:            r.Status,
		AmendmentFeedback: r.AmendmentFeedback,
		CreatedAt:         r.CreatedAt,
		ApprovedAt:        r.ApprovedAt,
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
			`INSERT INTO lg_tasks (workspace_id, title, reason, impl)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id, workspace_id, title, reason, impl, status, amendment_feedback, created_at, approved_at`,
			workspaceID, t.Title, t.Reason, impl,
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
		`SELECT id, workspace_id, title, reason, impl, status, amendment_feedback, created_at, approved_at
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

func (r *WorkspaceRepository) RejectTasks(ctx context.Context, rejections map[string]string) error {
	for taskID, feedback := range rejections {
		if _, err := r.db.ExecContext(ctx,
			`UPDATE lg_tasks SET status = 'rejected', amendment_feedback = $1 WHERE id = $2`,
			feedback, taskID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkspaceRepository) ApproveTasks(ctx context.Context, workspaceID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_tasks SET status = 'approved', approved_at = NOW()
		 WHERE workspace_id = $1 AND status = 'pending'`,
		workspaceID,
	)
	return err
}
