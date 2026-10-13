package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LDAi0/phonebook/internal/domain"
)

const uniqueViolation = "23505"

const searchWhere = `WHERE ($1::text = '' OR first_name ILIKE $2 OR last_name ILIKE $2
	OR (first_name || ' ' || last_name) ILIKE $2 OR phone LIKE $2)`

type ContactRepo struct {
	pool *pgxpool.Pool
}

func NewContactRepo(pool *pgxpool.Pool) *ContactRepo {
	return &ContactRepo{pool: pool}
}

func (r *ContactRepo) Create(ctx context.Context, c *domain.Contact) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO contacts (first_name, last_name, phone, email)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		c.FirstName, c.LastName, c.Phone, c.Email,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrPhoneTaken
		}
		return fmt.Errorf("insert contact: %w", err)
	}
	return nil
}

func (r *ContactRepo) GetByID(ctx context.Context, id int64) (*domain.Contact, error) {
	c := &domain.Contact{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, first_name, last_name, phone, email, created_at, updated_at
		 FROM contacts WHERE id = $1`, id,
	).Scan(&c.ID, &c.FirstName, &c.LastName, &c.Phone, &c.Email, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	return c, nil
}

func (r *ContactRepo) List(ctx context.Context, f domain.ListFilter) ([]domain.Contact, int, error) {
	pattern := "%" + escapeLike(f.Query) + "%"

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM contacts `+searchWhere, f.Query, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, first_name, last_name, phone, email, created_at, updated_at
		 FROM contacts `+searchWhere+`
		 ORDER BY last_name, first_name, id
		 LIMIT $3 OFFSET $4`,
		f.Query, pattern, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Contact, 0, f.Limit)
	for rows.Next() {
		var c domain.Contact
		if err := rows.Scan(&c.ID, &c.FirstName, &c.LastName, &c.Phone, &c.Email, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan contact: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate contacts: %w", err)
	}
	return items, total, nil
}

func (r *ContactRepo) Update(ctx context.Context, c *domain.Contact) error {
	err := r.pool.QueryRow(ctx,
		`UPDATE contacts
		 SET first_name = $1, last_name = $2, phone = $3, email = $4, updated_at = now()
		 WHERE id = $5
		 RETURNING created_at, updated_at`,
		c.FirstName, c.LastName, c.Phone, c.Email, c.ID,
	).Scan(&c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrPhoneTaken
		}
		return fmt.Errorf("update contact: %w", err)
	}
	return nil
}

func (r *ContactRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM contacts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// escapeLike neutralises LIKE wildcards in user input.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
