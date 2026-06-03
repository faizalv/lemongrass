package repository

import (
	"context"
	"time"

	"github.com/faizalv/lemongrass/modules/fs/entity"
)

type artifactRecord struct {
	ID        string    `db:"id"`
	ProjectID int64     `db:"project_id"`
	Type      string    `db:"type"`
	Name      string    `db:"name"`
	Content   string    `db:"content"`
	Version   int       `db:"version"`
	CreatedAt time.Time `db:"created_at"`
}

func toArtifactEntity(r artifactRecord) entity.Artifact {
	return entity.Artifact{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		Type:      r.Type,
		Name:      r.Name,
		Content:   r.Content,
		Version:   r.Version,
		CreatedAt: r.CreatedAt,
	}
}

func (r *FsRepository) ListArtifacts(ctx context.Context, projectID int64, typeFilter string) ([]entity.Artifact, error) {
	var recs []artifactRecord
	var err error
	if typeFilter != "" {
		err = r.db.SelectContext(ctx, &recs,
			`SELECT id, project_id, type, name, content, version, created_at
			 FROM lg_project_artifacts WHERE project_id = $1 AND type = $2
			 ORDER BY created_at DESC`,
			projectID, typeFilter,
		)
	} else {
		err = r.db.SelectContext(ctx, &recs,
			`SELECT id, project_id, type, name, content, version, created_at
			 FROM lg_project_artifacts WHERE project_id = $1
			 ORDER BY created_at DESC`,
			projectID,
		)
	}
	if err != nil {
		return nil, err
	}
	out := make([]entity.Artifact, len(recs))
	for i, rec := range recs {
		out[i] = toArtifactEntity(rec)
	}
	return out, nil
}

func (r *FsRepository) CreateArtifact(ctx context.Context, a entity.Artifact) (entity.Artifact, error) {
	var nextVersion int
	err := r.db.QueryRowxContext(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM lg_project_artifacts
		 WHERE project_id = $1 AND type = $2 AND name = $3`,
		a.ProjectID, a.Type, a.Name,
	).Scan(&nextVersion)
	if err != nil {
		return entity.Artifact{}, err
	}

	var rec artifactRecord
	err = r.db.QueryRowxContext(ctx,
		`INSERT INTO lg_project_artifacts (project_id, type, name, content, version)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, project_id, type, name, content, version, created_at`,
		a.ProjectID, a.Type, a.Name, a.Content, nextVersion,
	).StructScan(&rec)
	if err != nil {
		return entity.Artifact{}, err
	}
	return toArtifactEntity(rec), nil
}

func (r *FsRepository) DeleteArtifact(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_project_artifacts WHERE id = $1`, id)
	return err
}

func (r *FsRepository) DeleteArtifactsByProject(ctx context.Context, projectID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_project_artifacts WHERE project_id = $1`, projectID)
	return err
}
