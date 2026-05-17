package repository

import (
	"time"

	"github.com/faizalv/lemongrass/modules/fs/entity"
	"github.com/jmoiron/sqlx"
)

type projectRecord struct {
	ID        int64     `db:"id"`
	Path      string    `db:"path"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
}

func toEntity(r projectRecord) entity.Project {
	return entity.Project{
		ID:        r.ID,
		Path:      r.Path,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}

type FsRepository struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *FsRepository {
	return &FsRepository{db: db}
}

func (r *FsRepository) Save(path string) (entity.Project, error) {
	var rec projectRecord
	err := r.db.QueryRowx(
		`INSERT INTO lg_projects (path, status) VALUES ($1, 'pending')
		 ON CONFLICT (path) DO UPDATE SET status = 'pending'
		 RETURNING id, path, status, created_at`,
		path,
	).StructScan(&rec)
	if err != nil {
		return entity.Project{}, err
	}
	return toEntity(rec), nil
}

func (r *FsRepository) List() ([]entity.Project, error) {
	var recs []projectRecord
	if err := r.db.Select(&recs, `SELECT id, path, status, created_at FROM lg_projects ORDER BY created_at`); err != nil {
		return nil, err
	}
	projects := make([]entity.Project, len(recs))
	for i, rec := range recs {
		projects[i] = toEntity(rec)
	}
	return projects, nil
}

func (r *FsRepository) ListNonRemoved() ([]entity.Project, error) {
	var recs []projectRecord
	if err := r.db.Select(&recs,
		`SELECT id, path, status, created_at FROM lg_projects WHERE status != 'removed' ORDER BY created_at`,
	); err != nil {
		return nil, err
	}
	projects := make([]entity.Project, len(recs))
	for i, rec := range recs {
		projects[i] = toEntity(rec)
	}
	return projects, nil
}

func (r *FsRepository) UpdateStatus(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE lg_projects SET status = $1 WHERE id = $2`, status, id)
	return err
}
