package domain

import (
	"context"
	"time"
)

type PostRepository interface {
	Create(ctx context.Context, p *Post) error
	Update(ctx context.Context, p *Post) error
	BySlug(ctx context.Context, slug string, includeUnpublished bool) (Post, error)
	List(ctx context.Context, opts PostListOptions) ([]Post, error)
	Delete(ctx context.Context, id int64) error
	SetPostTags(ctx context.Context, postID int64, tagNames []string) error
}

type TagRepository interface {
	Cloud(ctx context.Context) ([]Tag, error)
	Suggest(ctx context.Context, query string, limit int) ([]Tag, error)
}

type UserRepository interface {
	ByUsername(ctx context.Context, username string) (User, error)
}

type SettingsRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type PostListOptions struct {
	Limit              int
	Offset             int
	TagSlug            string
	IncludeUnpublished bool
}

type Clock interface {
	Now() time.Time
}
