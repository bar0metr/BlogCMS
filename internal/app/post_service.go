package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"

	"blogcms/internal/domain"
)

type PostService struct {
	posts  domain.PostRepository
	tags   domain.TagRepository
	clock  domain.Clock
	mdPool *markdownPool
}

type PostServiceOptions struct {
	MarkdownPoolSize int
}

func NewPostService(posts domain.PostRepository, tags domain.TagRepository, clock domain.Clock, opt PostServiceOptions) *PostService {
	return &PostService{
		posts:  posts,
		tags:   tags,
		clock:  clock,
		mdPool: newMarkdownPool(max(1, opt.MarkdownPoolSize), func() goldmark.Markdown { return goldmark.New() }),
	}
}

type CreatePostInput struct {
	Title       string
	ContentMD   string
	TagsCSV     string
	IsPublished bool
}

func (s *PostService) Create(ctx context.Context, in CreatePostInput) (domain.Post, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return domain.Post{}, fmt.Errorf("%w: title is required", domain.ErrInvalidArgument)
	}

	slug := Slugify(title)
	html, err := s.renderMarkdown(in.ContentMD)
	if err != nil {
		return domain.Post{}, fmt.Errorf("render markdown: %w", err)
	}

	now := s.clock.Now()
	p := domain.Post{
		Title:       title,
		Slug:        slug,
		ContentMD:   in.ContentMD,
		ContentHTML: html,
		IsPublished: in.IsPublished,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if in.IsPublished {
		p.PublishedAt = &now
	}

	if err := s.posts.Create(ctx, &p); err != nil {
		return domain.Post{}, fmt.Errorf("create post: %w", err)
	}

	if err := s.posts.SetPostTags(ctx, p.ID, splitTags(in.TagsCSV)); err != nil {
		return domain.Post{}, fmt.Errorf("set post tags: %w", err)
	}

	return p, nil
}

type UpdatePostInput struct {
	ID          int64
	Title       string
	ContentMD   string
	TagsCSV     string
	IsPublished bool
}

func (s *PostService) Update(ctx context.Context, in UpdatePostInput) (domain.Post, error) {
	if in.ID <= 0 {
		return domain.Post{}, fmt.Errorf("%w: id is required", domain.ErrInvalidArgument)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return domain.Post{}, fmt.Errorf("%w: title is required", domain.ErrInvalidArgument)
	}

	html, err := s.renderMarkdown(in.ContentMD)
	if err != nil {
		return domain.Post{}, fmt.Errorf("render markdown: %w", err)
	}

	now := s.clock.Now()
	p := domain.Post{
		ID:          in.ID,
		Title:       title,
		Slug:        Slugify(title),
		ContentMD:   in.ContentMD,
		ContentHTML: html,
		IsPublished: in.IsPublished,
		UpdatedAt:   now,
	}
	if in.IsPublished {
		p.PublishedAt = &now
	}

	if err := s.posts.Update(ctx, &p); err != nil {
		return domain.Post{}, fmt.Errorf("update post: %w", err)
	}
	if err := s.posts.SetPostTags(ctx, p.ID, splitTags(in.TagsCSV)); err != nil {
		return domain.Post{}, fmt.Errorf("set post tags: %w", err)
	}
	return p, nil
}

func (s *PostService) GetBySlug(ctx context.Context, slug string, includeUnpublished bool) (domain.Post, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return domain.Post{}, fmt.Errorf("%w: slug is required", domain.ErrInvalidArgument)
	}
	p, err := s.posts.BySlug(ctx, slug, includeUnpublished)
	if err != nil {
		return domain.Post{}, err
	}
	return p, nil
}

func (s *PostService) List(ctx context.Context, opts domain.PostListOptions) ([]domain.Post, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 20
	}
	posts, err := s.posts.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	return posts, nil
}

func (s *PostService) TagCloud(ctx context.Context) ([]domain.Tag, error) {
	tags, err := s.tags.Cloud(ctx)
	if err != nil {
		return nil, fmt.Errorf("tag cloud: %w", err)
	}
	return tags, nil
}

func splitTags(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		t = strings.ToLower(t)
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func (s *PostService) renderMarkdown(input string) (string, error) {
	md := s.mdPool.Acquire()
	defer s.mdPool.Release(md)
	var b strings.Builder
	if err := md.Convert([]byte(input), &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
