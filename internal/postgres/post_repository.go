package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"blogcms/internal/app"
	"blogcms/internal/domain"
)

type PostRepository struct {
	db *sql.DB
}

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) Create(ctx context.Context, p *domain.Post) error {
	const q = `
INSERT INTO posts (title, slug, content_md, content_html, is_published, created_at, updated_at, published_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id;`
	return r.db.QueryRowContext(ctx, q,
		p.Title, p.Slug, p.ContentMD, p.ContentHTML, p.IsPublished, p.CreatedAt, p.UpdatedAt, p.PublishedAt,
	).Scan(&p.ID)
}

func (r *PostRepository) Update(ctx context.Context, p *domain.Post) error {
	const q = `
UPDATE posts
SET title=$1, slug=$2, content_md=$3, content_html=$4, is_published=$5, updated_at=$6, published_at=$7
WHERE id=$8;`
	res, err := r.db.ExecContext(ctx, q,
		p.Title, p.Slug, p.ContentMD, p.ContentHTML, p.IsPublished, p.UpdatedAt, p.PublishedAt, p.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PostRepository) BySlug(ctx context.Context, slug string, includeUnpublished bool) (domain.Post, error) {
	base := `
SELECT id, title, slug, content_md, content_html, is_published, created_at, updated_at, published_at
FROM posts
WHERE slug=$1`
	if !includeUnpublished {
		base += " AND is_published=true"
	}
	var p domain.Post
	err := r.db.QueryRowContext(ctx, base, slug).Scan(
		&p.ID, &p.Title, &p.Slug, &p.ContentMD, &p.ContentHTML, &p.IsPublished, &p.CreatedAt, &p.UpdatedAt, &p.PublishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Post{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Post{}, err
	}
	tags, err := r.postTags(ctx, p.ID)
	if err != nil {
		return domain.Post{}, fmt.Errorf("load tags: %w", err)
	}
	p.Tags = tags
	return p, nil
}

func (r *PostRepository) List(ctx context.Context, opts domain.PostListOptions) ([]domain.Post, error) {
	var args []any
	args = append(args, opts.Limit, opts.Offset)

	where := []string{"1=1"}
	if !opts.IncludeUnpublished {
		where = append(where, "p.is_published=true")
	}
	if opts.TagSlug != "" {
		where = append(where, "t.slug=$3")
		args = append(args, opts.TagSlug)
	}

	q := `
SELECT p.id, p.title, p.slug, p.content_md, p.content_html, p.is_published, p.created_at, p.updated_at, p.published_at
FROM posts p
`
	if opts.TagSlug != "" {
		q += `
JOIN post_tags pt ON pt.post_id=p.id
JOIN tags t ON t.id=pt.tag_id
`
	}
	q += "WHERE " + strings.Join(where, " AND ") + `
ORDER BY COALESCE(p.published_at, p.created_at) DESC
LIMIT $1 OFFSET $2;`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Post
	for rows.Next() {
		var p domain.Post
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Slug, &p.ContentMD, &p.ContentHTML, &p.IsPublished, &p.CreatedAt, &p.UpdatedAt, &p.PublishedAt,
		); err != nil {
			return nil, err
		}
		tags, err := r.postTags(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("load tags: %w", err)
		}
		p.Tags = tags
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM posts WHERE id=$1", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PostRepository) SetPostTags(ctx context.Context, postID int64, tagNames []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM post_tags WHERE post_id=$1", postID); err != nil {
		return err
	}
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		slug := app.Slugify(name)

		var tagID int64
		err := tx.QueryRowContext(ctx,
			`INSERT INTO tags (name, slug) VALUES ($1,$2)
			 ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name
			 RETURNING id;`,
			name, slug,
		).Scan(&tagID)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO post_tags (post_id, tag_id) VALUES ($1,$2)
			 ON CONFLICT DO NOTHING;`,
			postID, tagID,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *PostRepository) postTags(ctx context.Context, postID int64) ([]domain.Tag, error) {
	const q = `
SELECT t.id, t.name, t.slug
FROM tags t
JOIN post_tags pt ON pt.tag_id=t.id
WHERE pt.post_id=$1
ORDER BY t.name ASC;`
	rows, err := r.db.QueryContext(ctx, q, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
