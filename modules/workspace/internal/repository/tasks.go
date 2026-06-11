package repository

import (
	"context"
	"encoding/json"
	"time"

	lgentity "github.com/faizalv/lemongrass/modules/lg/entity"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type taskRecord struct {
	ID                string          `db:"id"`
	WorkspaceID       string          `db:"workspace_id"`
	TaskNumber        int             `db:"task_number"`
	Title             string          `db:"title"`
	Reason            string          `db:"reason"`
	Impl              json.RawMessage `db:"impl"`
	Status            string          `db:"status"`
	AmendmentFeedback string          `db:"amendment_feedback"`
	ExecutionStatus   string          `db:"execution_status"`
	ExecutionNotes    string          `db:"execution_notes"`
	ExecutionDiff     json.RawMessage `db:"execution_diff"`
	RejectionReason   string          `db:"rejection_reason"`
	StartedAt         *time.Time      `db:"started_at"`
	FinishedAt        *time.Time      `db:"finished_at"`
	CreatedAt         time.Time       `db:"created_at"`
	ApprovedAt        *time.Time      `db:"approved_at"`
}

func toTaskEntity(r taskRecord) entity.Task {
	return entity.Task{
		ID:                r.ID,
		WorkspaceID:       r.WorkspaceID,
		TaskNumber:        r.TaskNumber,
		Title:             r.Title,
		Reason:            r.Reason,
		Impl:              r.Impl,
		Status:            r.Status,
		AmendmentFeedback: r.AmendmentFeedback,
		ExecutionStatus:   r.ExecutionStatus,
		ExecutionNotes:    r.ExecutionNotes,
		ExecutionDiff:     r.ExecutionDiff,
		RejectionReason:   r.RejectionReason,
		StartedAt:         r.StartedAt,
		FinishedAt:        r.FinishedAt,
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
	for i, t := range tasks {
		impl := t.Impl
		if impl == nil {
			impl = json.RawMessage("[]")
		}
		var rec taskRecord
		err := r.db.QueryRowxContext(ctx,
			`INSERT INTO lg_tasks (workspace_id, title, reason, impl, task_number)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, workspace_id, task_number, title, reason, impl, status, amendment_feedback, created_at, approved_at`,
			workspaceID, t.Title, t.Reason, impl, i+1,
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
		`SELECT id, workspace_id, task_number, title, reason, impl, status, amendment_feedback,
		        execution_status, execution_notes, COALESCE(execution_diff, 'null'::jsonb) AS execution_diff,
		        rejection_reason, started_at, finished_at, created_at, approved_at
		 FROM lg_tasks WHERE workspace_id = $1 ORDER BY task_number ASC`,
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

func (r *WorkspaceRepository) GetTask(ctx context.Context, taskID string) (entity.Task, error) {
	var rec taskRecord
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, workspace_id, task_number, title, reason, impl, status, amendment_feedback,
		        execution_status, execution_notes, COALESCE(execution_diff, 'null'::jsonb) AS execution_diff,
		        rejection_reason, started_at, finished_at, created_at, approved_at
		 FROM lg_tasks WHERE id = $1`,
		taskID,
	).StructScan(&rec)
	if err != nil {
		return entity.Task{}, err
	}
	return toTaskEntity(rec), nil
}

func (r *WorkspaceRepository) GetTaskByNumber(ctx context.Context, workspaceID string, num int) (entity.Task, error) {
	var rec taskRecord
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, workspace_id, task_number, title, reason, impl, status, amendment_feedback,
		        execution_status, execution_notes, COALESCE(execution_diff, 'null'::jsonb) AS execution_diff,
		        rejection_reason, started_at, finished_at, created_at, approved_at
		 FROM lg_tasks WHERE workspace_id = $1 AND task_number = $2`,
		workspaceID, num,
	).StructScan(&rec)
	if err != nil {
		return entity.Task{}, err
	}
	return toTaskEntity(rec), nil
}

func (r *WorkspaceRepository) StartTask(ctx context.Context, taskID string, startedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_tasks SET execution_status = 'in_progress', started_at = $1 WHERE id = $2`,
		startedAt, taskID,
	)
	return err
}

func (r *WorkspaceRepository) FinishTask(ctx context.Context, taskID, notes string, diff []lgentity.FileDiff, finishedAt time.Time) error {
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		diffJSON = []byte("[]")
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE lg_tasks SET execution_status = 'done', execution_notes = $1, execution_diff = $2, finished_at = $3 WHERE id = $4`,
		notes, diffJSON, finishedAt, taskID,
	)
	return err
}

func (r *WorkspaceRepository) RejectTask(ctx context.Context, taskID, reason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_tasks SET execution_status = 'rejected', rejection_reason = $1,
		        execution_notes = '', execution_diff = NULL, started_at = NULL, finished_at = NULL
		 WHERE id = $2`,
		reason, taskID,
	)
	return err
}

func (r *WorkspaceRepository) GetRejectedTasks(ctx context.Context, workspaceID string) ([]entity.Task, error) {
	var recs []taskRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT id, workspace_id, task_number, title, reason, impl, status, amendment_feedback,
		        execution_status, execution_notes, COALESCE(execution_diff, 'null'::jsonb) AS execution_diff,
		        rejection_reason, started_at, finished_at, created_at, approved_at
		 FROM lg_tasks WHERE workspace_id = $1 AND execution_status = 'rejected' ORDER BY task_number ASC`,
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

func (r *WorkspaceRepository) CountInProgressTasks(ctx context.Context, workspaceID string) (int, error) {
	var count int
	err := r.db.QueryRowxContext(ctx,
		`SELECT COUNT(*) FROM lg_tasks WHERE workspace_id = $1 AND execution_status = 'in_progress'`,
		workspaceID,
	).Scan(&count)
	return count, err
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
