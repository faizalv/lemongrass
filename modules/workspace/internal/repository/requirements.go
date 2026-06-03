package repository

import (
	"context"
	"time"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

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

func (r *WorkspaceRepository) GetRequirement(ctx context.Context, workspaceID, reqID string) (entity.WorkspaceRequirement, error) {
	var rec requirementRecord
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, workspace_id, type, text_content, file_path, file_name, created_at
		 FROM lg_workspace_requirements WHERE id = $1 AND workspace_id = $2`,
		reqID, workspaceID,
	).StructScan(&rec)
	if err != nil {
		return entity.WorkspaceRequirement{}, err
	}
	return toRequirementEntity(rec), nil
}

func (r *WorkspaceRepository) CountRequirements(ctx context.Context, workspaceID string) (int, error) {
	var count int
	err := r.db.QueryRowxContext(ctx,
		`SELECT COUNT(*) FROM lg_workspace_requirements WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&count)
	return count, err
}
