package repository

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/modules/recon/entity"
	"github.com/lib/pq"
)

const nodeColumns = `id, project_id, file_path, line_start, line_end, package, symbol, kind,
	language, receiver, signature, exported, depends_on, status,
	description, return_type, content_hash, calls, branches, orphaned_at, explored_at, created_at`

func (r *ReconRepository) HasNodes(ctx context.Context, projectID int64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM lg_semantic_nodes WHERE project_id = $1 LIMIT 1`,
		projectID,
	).Scan(&count)
	return count > 0, err
}

func (r *ReconRepository) UpsertNodes(ctx context.Context, nodes []entity.SemanticNode, branch string) error {
	for _, n := range nodes {
		deps := pq.StringArray(n.DependsOn)
		if deps == nil {
			deps = pq.StringArray{}
		}
		calls := pq.StringArray(n.Calls)
		if calls == nil {
			calls = pq.StringArray{}
		}
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO lg_semantic_nodes
			  (project_id, file_path, line_start, line_end, package, symbol, kind, language,
			   receiver, signature, exported, depends_on, status, content_hash, calls, branches)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'unexplored',$13,$14,ARRAY[$15::text])
			ON CONFLICT (project_id, file_path, symbol, kind, content_hash) DO UPDATE SET
			  line_start   = EXCLUDED.line_start,
			  line_end     = EXCLUDED.line_end,
			  package      = EXCLUDED.package,
			  language     = EXCLUDED.language,
			  signature    = EXCLUDED.signature,
			  receiver     = EXCLUDED.receiver,
			  exported     = EXCLUDED.exported,
			  depends_on   = EXCLUDED.depends_on,
			  branches     = CASE
			    WHEN $15 = ANY(lg_semantic_nodes.branches) THEN lg_semantic_nodes.branches
			    ELSE array_append(lg_semantic_nodes.branches, $15)
			  END,
			  orphaned_at  = NULL,
			  calls = CASE
			    WHEN array_length(EXCLUDED.calls, 1) IS NOT NULL
			      THEN CASE
			        WHEN lg_semantic_nodes.status = 'explored'
			             AND array_length(lg_semantic_nodes.calls, 1) IS NOT NULL
			          THEN lg_semantic_nodes.calls
			        ELSE EXCLUDED.calls
			      END
			    ELSE lg_semantic_nodes.calls
			  END`,
			n.ProjectID, n.FilePath, n.LineStart, n.LineEnd,
			n.Package, n.Symbol, n.Kind, n.Language,
			n.Receiver, n.Signature, n.Exported, deps, n.ContentHash, calls, branch,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconRepository) InsertLgartNode(ctx context.Context, projectID int64, filePath, symbol, kind, language, contentHash, description, returnType string, calls []string, branches []string) error {
	c := pq.StringArray(calls)
	b := pq.StringArray(branches)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO lg_semantic_nodes
		  (project_id, file_path, line_start, line_end, package, symbol, kind, language,
		   exported, depends_on, status, content_hash, calls, description, return_type, branches)
		VALUES ($1,$2,0,0,'',$3,$4,$5,true,'{}'::text[],'explored',$6,$7,$8,$9,$10)
		ON CONFLICT (project_id, file_path, symbol, kind, content_hash) DO NOTHING`,
		projectID, filePath, symbol, kind, language, contentHash, c, description, returnType, b,
	)
	return err
}

func (r *ReconRepository) AddBranchToNode(ctx context.Context, projectID int64, filePath, symbol, kind, contentHash, branch string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE lg_semantic_nodes
		SET branches    = array_append(branches, $1),
		    orphaned_at = NULL
		WHERE project_id = $2
		  AND file_path = $3
		  AND symbol = $4
		  AND kind = $5
		  AND content_hash = $6
		  AND NOT ($1 = ANY(branches))`,
		branch, projectID, filePath, symbol, kind, contentHash,
	)
	return err
}

func (r *ReconRepository) RemoveBranchFromNode(ctx context.Context, projectID int64, filePath, symbol, kind, contentHash, branch string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE lg_semantic_nodes
		SET branches    = array_remove(branches, $1),
		    orphaned_at = CASE
		      WHEN array_length(array_remove(branches, $1), 1) IS NULL THEN NOW()
		      ELSE orphaned_at
		    END
		WHERE project_id = $2
		  AND file_path = $3
		  AND symbol = $4
		  AND kind = $5
		  AND content_hash = $6`,
		branch, projectID, filePath, symbol, kind, contentHash,
	)
	return err
}

func (r *ReconRepository) SetOrphanedAt(ctx context.Context, projectID int64, filePath, symbol, kind, contentHash string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE lg_semantic_nodes
		SET branches = '{}', orphaned_at = NOW()
		WHERE project_id = $1
		  AND file_path = $2
		  AND symbol = $3
		  AND kind = $4
		  AND content_hash = $5`,
		projectID, filePath, symbol, kind, contentHash,
	)
	return err
}

func (r *ReconRepository) DeleteExpiredOrphans(ctx context.Context, olderThan time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM lg_semantic_nodes WHERE orphaned_at IS NOT NULL AND orphaned_at < $1`,
		olderThan,
	)
	return err
}

// BulkStampBranch adds newBranch to every symbol row in the project that currently carries oldBranch.
// Used on same-tip branch switch: no rescan needed, all content is identical.
func (r *ReconRepository) BulkStampBranch(ctx context.Context, projectID int64, oldBranch, newBranch string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE lg_semantic_nodes
		SET branches = CASE
		      WHEN $3 = ANY(branches) THEN branches
		      ELSE array_append(branches, $3)
		    END
		WHERE project_id = $1
		  AND $2 = ANY(branches)`,
		projectID, oldBranch, newBranch,
	)
	return err
}

// BulkStampBranchForFiles adds newBranch to all symbol rows in the project for files NOT in the excludePaths
// set that already carry oldBranch. Used on diverged-tip branch switch to stamp unchanged files in bulk.
func (r *ReconRepository) BulkStampBranchForFiles(ctx context.Context, projectID int64, oldBranch, newBranch string, excludePaths []string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE lg_semantic_nodes
		SET branches = CASE
		      WHEN $3 = ANY(branches) THEN branches
		      ELSE array_append(branches, $3)
		    END
		WHERE project_id = $1
		  AND $2 = ANY(branches)
		  AND NOT (file_path = ANY($4))`,
		projectID, oldBranch, newBranch, pq.StringArray(excludePaths),
	)
	return err
}

func (r *ReconRepository) ListNodesInFilesWithBranch(ctx context.Context, projectID int64, filePaths []string, branch string) ([]entity.SemanticNode, error) {
	query := `SELECT ` + nodeColumns + `
	          FROM lg_semantic_nodes
	          WHERE project_id = $1
	            AND file_path = ANY($2)
	            AND $3 = ANY(branches)`
	var recs []nodeRecord
	if err := r.db.SelectContext(ctx, &recs, query, projectID, pq.StringArray(filePaths), branch); err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
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

func (r *ReconRepository) ListNodes(ctx context.Context, projectID int64, language, kind, status string) ([]entity.SemanticNode, error) {
	query := `SELECT ` + nodeColumns + `
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
		       COUNT(*) FILTER (WHERE status = 'explored') AS explored,
		       COUNT(*) FILTER (WHERE status = 'stale')    AS stale
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
		if err := rows.Scan(&c.Language, &c.Total, &c.Explored, &c.Stale); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ReconRepository) GetNode(ctx context.Context, projectID int64, filePath, symbol, kind string) (entity.SemanticNode, error) {
	var rec nodeRecord
	err := r.db.QueryRowxContext(ctx,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND file_path = $2 AND symbol = $3 AND kind = $4 AND status != 'removed'
		 LIMIT 1`,
		projectID, filePath, symbol, kind,
	).StructScan(&rec)
	if err != nil {
		return entity.SemanticNode{}, err
	}
	return toEntity(rec), nil
}

func (r *ReconRepository) GetNodeByHash(ctx context.Context, projectID int64, filePath, symbol, kind, contentHash string) (entity.SemanticNode, error) {
	var rec nodeRecord
	err := r.db.QueryRowxContext(ctx,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND file_path = $2 AND symbol = $3 AND kind = $4 AND content_hash = $5`,
		projectID, filePath, symbol, kind, contentHash,
	).StructScan(&rec)
	if err != nil {
		return entity.SemanticNode{}, err
	}
	return toEntity(rec), nil
}

func (r *ReconRepository) FindNodesBySymbol(ctx context.Context, projectID int64, filePath, symbol string) ([]entity.SemanticNode, error) {
	var recs []nodeRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND file_path = $2 AND symbol = $3 AND status != 'removed'`,
		projectID, filePath, symbol,
	)
	if err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
}

func (r *ReconRepository) AnnotateNode(ctx context.Context, projectID int64, filePath, symbol, kind, description, returnType string, calls []string) (int64, error) {
	c := pq.StringArray(calls)
	res, err := r.db.ExecContext(ctx,
		`UPDATE lg_semantic_nodes
		 SET description = $1, return_type = $2,
		     calls = CASE WHEN array_length($3::text[], 1) IS NOT NULL THEN $3 ELSE calls END,
		     status = 'explored', explored_at = NOW()
		 WHERE project_id = $4 AND file_path = $5 AND symbol = $6 AND kind = $7`,
		nullStr(description), nullStr(returnType), c, projectID, filePath, symbol, kind,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *ReconRepository) AnnotateNodeByHash(ctx context.Context, projectID int64, filePath, symbol, kind, contentHash, description, returnType string, calls []string) (int64, error) {
	c := pq.StringArray(calls)
	res, err := r.db.ExecContext(ctx,
		`UPDATE lg_semantic_nodes
		 SET description = $1, return_type = $2,
		     calls = CASE WHEN array_length($3::text[], 1) IS NOT NULL THEN $3 ELSE calls END,
		     status = 'explored', explored_at = NOW()
		 WHERE project_id = $4 AND file_path = $5 AND symbol = $6 AND kind = $7 AND content_hash = $8`,
		nullStr(description), nullStr(returnType), c, projectID, filePath, symbol, kind, contentHash,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *ReconRepository) SetNodeEmbeddingByHash(ctx context.Context, projectID int64, filePath, symbol, kind, contentHash string, embedding []float32) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_semantic_nodes SET embedding = $1::vector
		 WHERE project_id = $2 AND file_path = $3 AND symbol = $4 AND kind = $5 AND content_hash = $6`,
		formatVector(embedding), projectID, filePath, symbol, kind, contentHash,
	)
	return err
}

func (r *ReconRepository) GetProjectCoverage(ctx context.Context, projectID int64) (total, explored int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT
		   COUNT(*),
		   COUNT(*) FILTER (WHERE status = 'explored')
		 FROM lg_semantic_nodes
		 WHERE project_id = $1
		   AND status != 'removed'
		   AND kind NOT IN ('imports','dockerfile','makefile','ci-github','ci-gitlab','compose','config-yaml')`,
		projectID,
	).Scan(&total, &explored)
	return
}

func (r *ReconRepository) SetEmbedding(ctx context.Context, projectID int64, filePath, symbol string, embedding []float32) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE lg_semantic_nodes SET embedding = $1::vector
		 WHERE project_id = $2 AND file_path = $3 AND symbol = $4`,
		formatVector(embedding), projectID, filePath, symbol,
	)
	return err
}

func (r *ReconRepository) GetTreeCoverage(ctx context.Context, projectID int64, pathPrefix string) ([]entity.DirectoryCoverage, error) {
	query := `SELECT file_path, status FROM lg_semantic_nodes WHERE project_id = $1 AND status != 'removed'`
	args := []any{projectID}
	if pathPrefix != "" {
		args = append(args, pathPrefix+"%")
		query += ` AND file_path LIKE $2`
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type counts struct{ total, explored, stale int }
	dirs := make(map[string]*counts)
	for rows.Next() {
		var fp, st string
		if err := rows.Scan(&fp, &st); err != nil {
			return nil, err
		}
		dir := dirOf(fp)
		c, ok := dirs[dir]
		if !ok {
			c = &counts{}
			dirs[dir] = c
		}
		c.total++
		switch st {
		case "explored":
			c.explored++
		case "stale":
			c.stale++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]entity.DirectoryCoverage, 0, len(dirs))
	for dir, c := range dirs {
		out = append(out, entity.DirectoryCoverage{
			Dir:        dir,
			Total:      c.total,
			Explored:   c.explored,
			Stale:      c.stale,
			Unexplored: c.total - c.explored - c.stale,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

func dirOf(fp string) string {
	if i := strings.LastIndex(fp, "/"); i >= 0 {
		return fp[:i]
	}
	return "."
}

func (r *ReconRepository) ListByPathDirect(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, []entity.SubdirSummary, error) {
	pathPrefix = strings.TrimSuffix(strings.TrimPrefix(pathPrefix, "./"), "/")
	if pathPrefix == "." || pathPrefix == "" {
		return r.listRootDirect(ctx, projectID)
	}
	prefix := pathPrefix + "/"

	var recs []nodeRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1
		   AND file_path LIKE $2
		   AND file_path NOT LIKE $3
		   AND status != 'removed'
		 ORDER BY file_path, line_start`,
		projectID, prefix+"%", prefix+"%/%",
	)
	if err != nil {
		return nil, nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}

	type row struct {
		Name  string `db:"subdir_name"`
		Count int    `db:"symbol_count"`
	}
	var subdirRows []row
	prefixLen := len(prefix) + 1 // SUBSTR is 1-indexed
	err = r.db.SelectContext(ctx, &subdirRows,
		`SELECT
		    SPLIT_PART(SUBSTR(file_path, $3), '/', 1) AS subdir_name,
		    COUNT(*)::int AS symbol_count
		 FROM lg_semantic_nodes
		 WHERE project_id = $1
		   AND file_path LIKE $2
		   AND STRPOS(SUBSTR(file_path, $3), '/') > 0
		   AND status != 'removed'
		 GROUP BY subdir_name
		 ORDER BY subdir_name`,
		projectID, prefix+"%", prefixLen,
	)
	if err != nil {
		return nil, nil, err
	}
	subdirs := make([]entity.SubdirSummary, len(subdirRows))
	for i, r := range subdirRows {
		subdirs[i] = entity.SubdirSummary{
			Path:  pathPrefix + "/" + r.Name,
			Count: r.Count,
		}
	}
	return nodes, subdirs, nil
}

func (r *ReconRepository) listRootDirect(ctx context.Context, projectID int64) ([]entity.SemanticNode, []entity.SubdirSummary, error) {
	var recs []nodeRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1
		   AND file_path NOT LIKE '%/%'
		   AND status != 'removed'
		 ORDER BY file_path, line_start`,
		projectID,
	)
	if err != nil {
		return nil, nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}

	type row struct {
		Name  string `db:"subdir_name"`
		Count int    `db:"symbol_count"`
	}
	var subdirRows []row
	err = r.db.SelectContext(ctx, &subdirRows,
		`SELECT
		    SPLIT_PART(file_path, '/', 1) AS subdir_name,
		    COUNT(*)::int AS symbol_count
		 FROM lg_semantic_nodes
		 WHERE project_id = $1
		   AND file_path LIKE '%/%'
		   AND status != 'removed'
		 GROUP BY subdir_name
		 ORDER BY subdir_name`,
		projectID,
	)
	if err != nil {
		return nil, nil, err
	}
	subdirs := make([]entity.SubdirSummary, len(subdirRows))
	for i, r := range subdirRows {
		subdirs[i] = entity.SubdirSummary{Path: r.Name, Count: r.Count}
	}
	return nodes, subdirs, nil
}

func (r *ReconRepository) ListUnembedded(ctx context.Context, limit int) ([]entity.SemanticNode, error) {
	var recs []nodeRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE embedding IS NULL AND status != 'removed'
		   AND (description IS NULL OR description = '')
		   AND kind NOT IN ('imports', 'commented-block', 'vue-template', 'vue-style')
		 ORDER BY created_at ASC,
		          CASE WHEN status = 'unexplored' THEN 0 ELSE 1 END
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
}

func (r *ReconRepository) SearchByVector(ctx context.Context, projectID int64, embedding []float32, limit int) ([]entity.SemanticNode, error) {
	var recs []nodeRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND status != 'removed' AND embedding IS NOT NULL
		 ORDER BY embedding <=> $2::vector
		 LIMIT $3`,
		projectID, formatVector(embedding), limit,
	)
	if err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
}

func (r *ReconRepository) SearchByFTS(ctx context.Context, projectID int64, query string, limit int) ([]entity.SemanticNode, error) {
	var recs []nodeRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND status != 'removed' AND description IS NOT NULL AND embedding IS NULL
		   AND to_tsvector('english', description) @@ plainto_tsquery('english', $2)
		 ORDER BY ts_rank(to_tsvector('english', description), plainto_tsquery('english', $2)) DESC
		 LIMIT $3`,
		projectID, query, limit,
	)
	if err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
}

func (r *ReconRepository) ListAllNodesByPrefix(ctx context.Context, projectID int64, pathPrefix string) ([]entity.SemanticNode, error) {
	query := `SELECT ` + nodeColumns + `
	          FROM lg_semantic_nodes
	          WHERE project_id = $1 AND status != 'removed'`
	args := []any{projectID}
	if pathPrefix != "" && pathPrefix != "." {
		args = append(args, pathPrefix+"%")
		query += ` AND file_path LIKE $2`
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

func (r *ReconRepository) ListFileNodes(ctx context.Context, projectID int64, filePath string) ([]entity.SemanticNode, error) {
	query := `SELECT ` + nodeColumns + `
	          FROM lg_semantic_nodes
	          WHERE project_id = $1
	            AND file_path = $2
	            AND line_start > 0
	            AND line_end >= line_start
	            AND kind NOT IN ('class', 'trait', 'imports', 'vue-template', 'vue-style', 'commented-block')
	          ORDER BY line_start`
	var recs []nodeRecord
	if err := r.db.SelectContext(ctx, &recs, query, projectID, filePath); err != nil {
		return nil, err
	}
	nodes := make([]entity.SemanticNode, len(recs))
	for i, rec := range recs {
		nodes[i] = toEntity(rec)
	}
	return nodes, nil
}

func (r *ReconRepository) GetRelated(ctx context.Context, projectID int64, filePath, symbol, kind string) (callees, callers []entity.SemanticNode, err error) {
	var callSymbols pq.StringArray
	scanErr := r.db.QueryRowContext(ctx,
		`SELECT calls FROM lg_semantic_nodes WHERE project_id = $1 AND file_path = $2 AND symbol = $3 AND kind = $4`,
		projectID, filePath, symbol, kind,
	).Scan(&callSymbols)
	if scanErr != nil && scanErr != sql.ErrNoRows {
		err = scanErr
		return
	}

	if len(callSymbols) > 0 {
		var recs []nodeRecord
		if err = r.db.SelectContext(ctx, &recs,
			`SELECT `+nodeColumns+`
			 FROM lg_semantic_nodes
			 WHERE project_id = $1 AND symbol = ANY($2) AND status = 'explored'`,
			projectID, pq.Array(callSymbols),
		); err != nil {
			return
		}
		callees = make([]entity.SemanticNode, len(recs))
		for i, rec := range recs {
			callees[i] = toEntity(rec)
		}
	}

	var callerRecs []nodeRecord
	if err = r.db.SelectContext(ctx, &callerRecs,
		`SELECT `+nodeColumns+`
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND $2 = ANY(calls) AND status = 'explored'`,
		projectID, symbol,
	); err != nil {
		return
	}
	callers = make([]entity.SemanticNode, len(callerRecs))
	for i, rec := range callerRecs {
		callers[i] = toEntity(rec)
	}
	return
}

func (r *ReconRepository) DeleteNodesByFilePaths(ctx context.Context, projectID int64, filePaths []string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM lg_semantic_nodes WHERE project_id = $1 AND file_path = ANY($2)`,
		projectID, pq.StringArray(filePaths))
	return err
}

func (r *ReconRepository) GetEmbedPending(ctx context.Context, projectID int64) (pending, total int, err error) {
	err = r.db.QueryRowContext(ctx,
		`SELECT
		   COUNT(*),
		   COUNT(*) FILTER (WHERE embedding IS NULL
		                    AND kind NOT IN ('imports', 'commented-block', 'vue-template', 'vue-style'))
		 FROM lg_semantic_nodes
		 WHERE project_id = $1 AND status != 'removed'`,
		projectID,
	).Scan(&total, &pending)
	return
}

func (r *ReconRepository) GetStaleCount(ctx context.Context, projectID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM lg_semantic_nodes WHERE project_id = $1 AND status = 'stale'`,
		projectID).Scan(&count)
	return count, err
}

func (r *ReconRepository) CheckNodeOverlap(ctx context.Context, projectID int64, keys []string) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lg_semantic_nodes
		 WHERE project_id = $1
		   AND status != 'removed'
		   AND (file_path || ':' || symbol || ':' || kind) = ANY($2)`,
		projectID, pq.Array(keys),
	).Scan(&count)
	return count, err
}

// PruneSuperseded deletes unexplored/stale rows that have been superseded by an explored
// row with the same (project, file, symbol, kind) but a different content_hash.
func (r *ReconRepository) PruneSuperseded(ctx context.Context, projectID int64) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM lg_semantic_nodes old
		WHERE old.project_id = $1
		  AND old.status IN ('unexplored', 'stale')
		  AND EXISTS (
		    SELECT 1 FROM lg_semantic_nodes e
		    WHERE e.project_id  = old.project_id
		      AND e.file_path   = old.file_path
		      AND e.symbol      = old.symbol
		      AND e.kind        = old.kind
		      AND e.content_hash != old.content_hash
		      AND e.status = 'explored'
		  )`,
		projectID,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// PruneOrphans deletes orphaned rows (branches = []) for a project that were orphaned
// before the given cutoff.
func (r *ReconRepository) PruneOrphans(ctx context.Context, projectID int64, olderThan time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM lg_semantic_nodes
		WHERE project_id = $1
		  AND orphaned_at IS NOT NULL
		  AND orphaned_at < $2`,
		projectID, olderThan,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
