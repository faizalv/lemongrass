package repository

import (
	"context"
	"strconv"
	"strings"

	lge "github.com/faizalv/lemongrass/modules/lg/entity"
	"github.com/jmoiron/sqlx"
)

type InterimRepository struct {
	db *sqlx.DB
}

func NewInterim(db *sqlx.DB) *InterimRepository {
	return &InterimRepository{db: db}
}

func (r *InterimRepository) DropInterim(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM lg_codebase_interim WHERE session_id = $1`, sessionID)
	return err
}

func (r *InterimRepository) InsertChunk(ctx context.Context, sessionID, filePath string, chunkIndex, lineStart, lineEnd int, content string, embedding []float32) error {
	var embArg interface{}
	if len(embedding) > 0 {
		embArg = formatVector(embedding)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO lg_codebase_interim (session_id, file_path, chunk_index, content, embedding, line_start, line_end)
		 VALUES ($1, $2, $3, $4, $5::vector, $6, $7)`,
		sessionID, filePath, chunkIndex, content, embArg, lineStart, lineEnd,
	)
	return err
}

func (r *InterimRepository) QueryInterim(ctx context.Context, sessionID string, embedding []float32, limit int) ([]lge.InterimChunk, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT file_path, chunk_index, content, line_start, line_end
		 FROM lg_codebase_interim
		 WHERE session_id = $1 AND embedding IS NOT NULL
		 ORDER BY embedding <=> $2::vector
		 LIMIT $3`,
		sessionID, formatVector(embedding), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *InterimRepository) SearchInterim(ctx context.Context, sessionID, pattern string) ([]lge.InterimChunk, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT file_path, chunk_index, content, line_start, line_end
		 FROM lg_codebase_interim
		 WHERE session_id = $1 AND content ILIKE '%' || $2 || '%'
		 ORDER BY chunk_index`,
		sessionID, pattern,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (r *InterimRepository) HasInterim(ctx context.Context, sessionID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM lg_codebase_interim WHERE session_id = $1 LIMIT 1`,
		sessionID,
	).Scan(&n)
	return n > 0, err
}

func scanChunks(rows interface {
	Next() bool
	Scan(...interface{}) error
	Close() error
}) ([]lge.InterimChunk, error) {
	var chunks []lge.InterimChunk
	for rows.Next() {
		var c lge.InterimChunk
		if err := rows.Scan(&c.FilePath, &c.ChunkIndex, &c.Content, &c.LineStart, &c.LineEnd); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

func formatVector(v []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(f), 'f', 8, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}
