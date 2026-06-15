package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryoku/kubegate/internal/domain"
)

// ErrSlugConflict is returned when a product with the same slug already exists.
var ErrSlugConflict = errors.New("slug already exists")

// ErrNotFound is returned when a requested product does not exist.
var ErrNotFound = errors.New("product not found")

// ListOptions controls how List filters results.
type ListOptions struct {
	// SlugAllowlist, when non-nil, restricts results to products whose slug is in the set.
	// A nil map means "return all" (admin path).
	SlugAllowlist   map[string]struct{}
	IncludeArchived bool
}

// ProductStore is the persistence interface for products.
type ProductStore interface {
	Create(ctx context.Context, p *domain.Product) error
	List(ctx context.Context, opts ListOptions) ([]domain.Product, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Product, error)
	// GetByID returns the product with the given UUID, or ErrNotFound.
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	Update(ctx context.Context, slug string, name, description string) (*domain.Product, error)
	Archive(ctx context.Context, slug string) error
	// GetTagConvention returns the tag convention regex for the product identified by slug.
	// It returns nil if no product-level override is set. Returns ErrNotFound if no product
	// with that slug exists.
	GetTagConvention(ctx context.Context, slug string) (*string, error)
	// SetTagConvention sets the tag convention regex for the product identified by slug.
	// Returns ErrNotFound if no product with that slug exists.
	SetTagConvention(ctx context.Context, slug, regex string) error
	// ClearTagConvention removes the product-level tag convention override, reverting to
	// the global default. Returns ErrNotFound if no active product with that slug exists.
	ClearTagConvention(ctx context.Context, slug string) error
}

type pgxProductStore struct {
	pool *pgxpool.Pool
}

// NewProductStore returns a ProductStore backed by the given pgxpool.
func NewProductStore(pool *pgxpool.Pool) ProductStore {
	return &pgxProductStore{pool: pool}
}

func (s *pgxProductStore) Create(ctx context.Context, p *domain.Product) error {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO products (name, slug, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		p.Name, p.Slug, p.Description,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return ErrSlugConflict
		}
		return fmt.Errorf("create product: %w", err)
	}
	return nil
}

func (s *pgxProductStore) List(ctx context.Context, opts ListOptions) ([]domain.Product, error) {
	var (
		query string
		args  []any
	)

	base := `SELECT id, name, slug, description, archived_at, created_at, tag_convention_regex FROM products`
	var conditions []string

	if !opts.IncludeArchived {
		conditions = append(conditions, "archived_at IS NULL")
	}

	if opts.SlugAllowlist != nil {
		if len(opts.SlugAllowlist) == 0 {
			return []domain.Product{}, nil
		}
		slugs := make([]string, 0, len(opts.SlugAllowlist))
		for s := range opts.SlugAllowlist {
			slugs = append(slugs, s)
		}
		args = append(args, slugs)
		conditions = append(conditions, fmt.Sprintf("slug = ANY($%d)", len(args)))
	}

	if len(conditions) > 0 {
		query = base + " WHERE " + strings.Join(conditions, " AND ")
	} else {
		query = base
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var products []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.ArchivedAt, &p.CreatedAt, &p.TagConventionRegex); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list products rows: %w", err)
	}
	if products == nil {
		return []domain.Product{}, nil
	}
	return products, nil
}

func (s *pgxProductStore) GetBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	var p domain.Product
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, description, archived_at, created_at, tag_convention_regex FROM products WHERE slug = $1`,
		slug,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.ArchivedAt, &p.CreatedAt, &p.TagConventionRegex)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get product by slug: %w", err)
	}
	return &p, nil
}

func (s *pgxProductStore) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	var p domain.Product
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, slug, description, archived_at, created_at, tag_convention_regex FROM products WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.ArchivedAt, &p.CreatedAt, &p.TagConventionRegex)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get product by id: %w", err)
	}
	return &p, nil
}

func (s *pgxProductStore) Update(ctx context.Context, slug string, name, description string) (*domain.Product, error) {
	var p domain.Product
	err := s.pool.QueryRow(ctx,
		`UPDATE products
		 SET name = $1, description = $2
		 WHERE slug = $3 AND archived_at IS NULL
		 RETURNING id, name, slug, description, archived_at, created_at`,
		name, description, slug,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.ArchivedAt, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update product: %w", err)
	}
	return &p, nil
}

func (s *pgxProductStore) Archive(ctx context.Context, slug string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE products SET archived_at = NOW() WHERE slug = $1 AND archived_at IS NULL`,
		slug,
	)
	if err != nil {
		return fmt.Errorf("archive product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either not found or already archived — treat as not found for anti-enumeration.
		return ErrNotFound
	}
	return nil
}

func (s *pgxProductStore) GetTagConvention(ctx context.Context, slug string) (*string, error) {
	var regex *string
	err := s.pool.QueryRow(ctx,
		`SELECT tag_convention_regex FROM products WHERE slug = $1 AND archived_at IS NULL`,
		slug,
	).Scan(&regex)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get tag convention: %w", err)
	}
	return regex, nil
}

func (s *pgxProductStore) SetTagConvention(ctx context.Context, slug, regex string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE products SET tag_convention_regex = $1 WHERE slug = $2 AND archived_at IS NULL`,
		regex, slug,
	)
	if err != nil {
		return fmt.Errorf("set tag convention: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgxProductStore) ClearTagConvention(ctx context.Context, slug string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE products SET tag_convention_regex = NULL WHERE slug = $1 AND archived_at IS NULL`,
		slug,
	)
	if err != nil {
		return fmt.Errorf("clear tag convention: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
