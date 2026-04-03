package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

type mimeRow struct {
	ID        int16  `db:"id"`
	Name      string `db:"name"`
	Extension string `db:"extension"`
}

func toMIMEType(r mimeRow) domain.MIMEType {
	return domain.MIMEType{
		ID:        r.ID,
		Name:      r.Name,
		Extension: r.Extension,
	}
}

// MimeRepo implements port.MimeRepo using PostgreSQL.
type MimeRepo struct {
	pool *pgxpool.Pool
}

// NewMimeRepo creates a MimeRepo backed by pool.
func NewMimeRepo(pool *pgxpool.Pool) *MimeRepo {
	return &MimeRepo{pool: pool}
}

var _ port.MimeRepo = (*MimeRepo)(nil)

func (r *MimeRepo) List(ctx context.Context) ([]domain.MIMEType, error) {
	const sql = `SELECT id, name, extension FROM core.mime_types ORDER BY name`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("MimeRepo.List: %w", err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[mimeRow])
	if err != nil {
		return nil, fmt.Errorf("MimeRepo.List scan: %w", err)
	}

	result := make([]domain.MIMEType, len(collected))
	for i, row := range collected {
		result[i] = toMIMEType(row)
	}
	return result, nil
}

func (r *MimeRepo) GetByID(ctx context.Context, id int16) (*domain.MIMEType, error) {
	const sql = `SELECT id, name, extension FROM core.mime_types WHERE id = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, id)
	if err != nil {
		return nil, fmt.Errorf("MimeRepo.GetByID: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[mimeRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("MimeRepo.GetByID scan: %w", err)
	}
	m := toMIMEType(row)
	return &m, nil
}

func (r *MimeRepo) GetByName(ctx context.Context, name string) (*domain.MIMEType, error) {
	const sql = `SELECT id, name, extension FROM core.mime_types WHERE name = $1`

	q := connOrTx(ctx, r.pool)
	rows, err := q.Query(ctx, sql, name)
	if err != nil {
		return nil, fmt.Errorf("MimeRepo.GetByName: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[mimeRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUnsupportedMIME
		}
		return nil, fmt.Errorf("MimeRepo.GetByName scan: %w", err)
	}
	m := toMIMEType(row)
	return &m, nil
}
