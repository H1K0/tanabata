package postgres

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// ---------------------------------------------------------------------------
// DuplicatePairRepo
// ---------------------------------------------------------------------------

// DuplicatePairRepo implements port.DuplicatePairRepo using PostgreSQL.
type DuplicatePairRepo struct {
	pool *pgxpool.Pool
}

// NewDuplicatePairRepo creates a DuplicatePairRepo backed by pool.
func NewDuplicatePairRepo(pool *pgxpool.Pool) *DuplicatePairRepo {
	return &DuplicatePairRepo{pool: pool}
}

var _ port.DuplicatePairRepo = (*DuplicatePairRepo)(nil)

// ReplaceAll atomically replaces the entire pairs table with the given set.
// The rescan recomputes pairs from scratch, so a full DELETE + COPY is both
// correct and the simplest way to drop pairs that no longer qualify.
func (r *DuplicatePairRepo) ReplaceAll(ctx context.Context, pairs []domain.DuplicatePair) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("DuplicatePairRepo.ReplaceAll begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	if _, err := tx.Exec(ctx, `DELETE FROM data.duplicate_pairs`); err != nil {
		return fmt.Errorf("DuplicatePairRepo.ReplaceAll delete: %w", err)
	}

	if len(pairs) > 0 {
		rows := make([][]any, len(pairs))
		for i, p := range pairs {
			rows[i] = []any{p.FileA, p.FileB, int16(p.Distance)}
		}
		if _, err := tx.CopyFrom(ctx,
			pgx.Identifier{"data", "duplicate_pairs"},
			[]string{"file_a", "file_b", "distance"},
			pgx.CopyFromRows(rows),
		); err != nil {
			return fmt.Errorf("DuplicatePairRepo.ReplaceAll copy: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("DuplicatePairRepo.ReplaceAll commit: %w", err)
	}
	return nil
}

type pairRow struct {
	FileA    uuid.UUID `db:"file_a"`
	FileB    uuid.UUID `db:"file_b"`
	Distance int16     `db:"distance"`
}

// ListVisible returns every stored pair where both files are live (not trashed),
// the pair is not dismissed, and — for non-admins — both files are visible to the
// viewer under the private-by-default model. This is the input to clustering.
func (r *DuplicatePairRepo) ListVisible(ctx context.Context, viewerID int16, isAdmin bool) ([]domain.DuplicatePair, error) {
	args := make([]any, 0, 4)
	n := 1
	aclWhere := ""
	if !isAdmin {
		var ca, cb string
		ca, n, args = aclVisibilityCond("fa", objTypeFile, viewerID, n, args)
		cb, n, args = aclVisibilityCond("fb", objTypeFile, viewerID, n, args)
		aclWhere = "AND " + ca + " AND " + cb
	}

	sqlStr := fmt.Sprintf(`
        SELECT p.file_a, p.file_b, p.distance
        FROM data.duplicate_pairs p
        JOIN data.files fa ON fa.id = p.file_a AND fa.is_deleted = false
        JOIN data.files fb ON fb.id = p.file_b AND fb.is_deleted = false
        WHERE NOT EXISTS (
            SELECT 1 FROM data.duplicate_dismissals d
            WHERE d.file_a = p.file_a AND d.file_b = p.file_b
        )
        %s
        ORDER BY p.file_a, p.file_b`, aclWhere)

	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("DuplicatePairRepo.ListVisible: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[pairRow])
	if err != nil {
		return nil, fmt.Errorf("DuplicatePairRepo.ListVisible scan: %w", err)
	}
	out := make([]domain.DuplicatePair, len(collected))
	for i, row := range collected {
		out[i] = domain.DuplicatePair{FileA: row.FileA, FileB: row.FileB, Distance: int(row.Distance)}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// DismissalRepo
// ---------------------------------------------------------------------------

// DismissalRepo implements port.DismissalRepo using PostgreSQL.
type DismissalRepo struct {
	pool *pgxpool.Pool
}

// NewDismissalRepo creates a DismissalRepo backed by pool.
func NewDismissalRepo(pool *pgxpool.Pool) *DismissalRepo {
	return &DismissalRepo{pool: pool}
}

var _ port.DismissalRepo = (*DismissalRepo)(nil)

// Add records a pair as "not a duplicate". The two ids are stored in canonical
// (file_a < file_b) order to match the table's CHECK and avoid (a,b)/(b,a)
// duplicates; a repeated dismissal is a no-op.
func (r *DismissalRepo) Add(ctx context.Context, a, b uuid.UUID, userID int16) error {
	if bytes.Compare(a[:], b[:]) > 0 {
		a, b = b, a
	}
	const sqlStr = `
        INSERT INTO data.duplicate_dismissals (file_a, file_b, dismissed_by)
        VALUES ($1, $2, $3)
        ON CONFLICT (file_a, file_b) DO NOTHING`
	q := connOrTx(ctx, r.pool)
	if _, err := q.Exec(ctx, sqlStr, a, b, userID); err != nil {
		return fmt.Errorf("DismissalRepo.Add: %w", err)
	}
	return nil
}
