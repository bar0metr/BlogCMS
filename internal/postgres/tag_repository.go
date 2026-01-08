package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"blogcms/internal/domain"
)

type TagRepository struct {
	db *sql.DB
}

func NewTagRepository(db *sql.DB) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) Cloud(ctx context.Context) ([]domain.Tag, error) {
	const q = `
SELECT t.id, t.name, t.slug, COUNT(pt.post_id) AS used
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id=t.id
LEFT JOIN posts p ON p.id=pt.post_id AND p.is_published=true
GROUP BY t.id, t.name, t.slug
HAVING COUNT(pt.post_id) > 0
ORDER BY used DESC, t.name ASC;`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query tag cloud: %w", err)
	}
	defer rows.Close()

	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Used); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TagRepository) Suggest(ctx context.Context, query string, limit int) ([]domain.Tag, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// Prefix match on tag name (case-insensitive). We do NOT filter by
	// published usage; admin wants suggestions from the full tag set.
	const sqlq = `
SELECT id, name, slug
FROM tags
WHERE name ILIKE $1
ORDER BY name ASC
LIMIT $2;`

	rows, err := r.db.QueryContext(ctx, sqlq, q+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("query tag suggestions: %w", err)
	}
	defer rows.Close()

	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
