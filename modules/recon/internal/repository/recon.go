package repository

import (
	"context"
	"time"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type ReconRepository struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *ReconRepository {
	return &ReconRepository{db: db}
}

type nodeRecord struct {
	ID          string         `db:"id"`
	ProjectID   int64          `db:"project_id"`
	FilePath    string         `db:"file_path"`
	LineStart   int            `db:"line_start"`
	LineEnd     int            `db:"line_end"`
	Package     string         `db:"package"`
	Symbol      string         `db:"symbol"`
	Kind        string         `db:"kind"`
	Language    string         `db:"language"`
	Receiver    *string        `db:"receiver"`
	Signature   *string        `db:"signature"`
	Exported    bool           `db:"exported"`
	DependsOn   pq.StringArray `db:"depends_on"`
	Status      string         `db:"status"`
	Description *string        `db:"description"`
	ReturnType  *string        `db:"return_type"`
	ExploredAt  *time.Time     `db:"explored_at"`
	CreatedAt   time.Time      `db:"created_at"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toEntity(r nodeRecord) entity.SemanticNode {
	return entity.SemanticNode{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		FilePath:    r.FilePath,
		LineStart:   r.LineStart,
		LineEnd:     r.LineEnd,
		Package:     r.Package,
		Symbol:      r.Symbol,
		Kind:        r.Kind,
		Language:    r.Language,
		Receiver:    deref(r.Receiver),
		Signature:   deref(r.Signature),
		Exported:    r.Exported,
		DependsOn:   []string(r.DependsOn),
		Status:      r.Status,
		Description: deref(r.Description),
		ReturnType:  deref(r.ReturnType),
		ExploredAt:  r.ExploredAt,
		CreatedAt:   r.CreatedAt,
	}
}

func (r *ReconRepository) ProjectDir(ctx context.Context, projectID int64) (string, error) {
	var path string
	err := r.db.QueryRowContext(ctx, `SELECT path FROM lg_projects WHERE id = $1`, projectID).Scan(&path)
	return path, err
}

func (r *ReconRepository) DeleteByProject(ctx context.Context, projectID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_semantic_nodes WHERE project_id = $1`, projectID)
	return err
}

func (r *ReconRepository) HasNodes(ctx context.Context, projectID int64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM lg_semantic_nodes WHERE project_id = $1 LIMIT 1`,
		projectID,
	).Scan(&count)
	return count > 0, err
}

func (r *ReconRepository) UpsertNodes(ctx context.Context, nodes []entity.SemanticNode) error {
	for _, n := range nodes {
		deps := pq.StringArray(n.DependsOn)
		if deps == nil {
			deps = pq.StringArray{}
		}
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO lg_semantic_nodes
			  (project_id, file_path, line_start, line_end, package, symbol, kind, language,
			   receiver, signature, exported, depends_on, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'unexplored')
			ON CONFLICT (project_id, file_path, symbol, kind) DO UPDATE SET
			  line_start  = EXCLUDED.line_start,
			  line_end    = EXCLUDED.line_end,
			  signature   = EXCLUDED.signature,
			  depends_on  = EXCLUDED.depends_on`,
			n.ProjectID, n.FilePath, n.LineStart, n.LineEnd,
			n.Package, n.Symbol, n.Kind, n.Language,
			n.Receiver, n.Signature, n.Exported, deps,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconRepository) MarkRemoved(ctx context.Context, projectID int64, parsedPaths []string, ignoredExisting []string) error {
	alive := append(parsedPaths, ignoredExisting...)
	_, err := r.db.ExecContext(ctx, `
		UPDATE lg_semantic_nodes
		SET status = 'removed'
		WHERE project_id = $1
		  AND status != 'removed'
		  AND NOT (file_path = ANY($2))`,
		projectID, pq.StringArray(alive),
	)
	return err
}

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

func (r *ReconRepository) ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error) {
	query := `SELECT id, project_id, file_path, line_start, line_end, package, symbol, kind,
	                 language, receiver, signature, exported, depends_on, status,
	                 description, return_type, explored_at, created_at
	          FROM lg_semantic_nodes
	          WHERE project_id = $1 AND status != 'removed'`
	args := []any{projectID}

	if language != "" {
		args = append(args, language)
		query += ` AND language = $` + itoa(len(args))
	}
	if kind != "" {
		args = append(args, kind)
		query += ` AND kind = $` + itoa(len(args))
	}
	if status != "" {
		args = append(args, status)
		query += ` AND status = $` + itoa(len(args))
	}
	query += ` ORDER BY file_path, line_start`

	var recs []nodeRecord
	if err := r.db.SelectContext(ctx, &recs, query, args...); err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
}

func (r *ReconRepository) GetCoverage(ctx context.Context, projectID int64) ([]entity.LangCoverage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT language,
		       COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE status = 'explored') AS explored
		FROM lg_semantic_nodes
		WHERE project_id = $1 AND status != 'removed'
		GROUP BY language
		ORDER BY language`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.LangCoverage
	for rows.Next() {
		var c entity.LangCoverage
		if err := rows.Scan(&c.Language, &c.Total, &c.Explored); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	return itoa(n/10) + string(digits[n%10])
}
