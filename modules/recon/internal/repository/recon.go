package repository

import (
	"strconv"
	"strings"
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
	ContentHash *string        `db:"content_hash"`
	Calls       pq.StringArray `db:"calls"`
	Branches    pq.StringArray `db:"branches"`
	OrphanedAt  *time.Time     `db:"orphaned_at"`
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
	calls := []string(r.Calls)
	if calls == nil {
		calls = []string{}
	}
	branches := []string(r.Branches)
	if branches == nil {
		branches = []string{}
	}
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
		ContentHash: deref(r.ContentHash),
		Calls:       calls,
		Branches:    branches,
		OrphanedAt:  r.OrphanedAt,
		ExploredAt:  r.ExploredAt,
		CreatedAt:   r.CreatedAt,
	}
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func formatVector(v []float32) string {
	sb := strings.Builder{}
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

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	return itoa(n/10) + string(digits[n%10])
}
