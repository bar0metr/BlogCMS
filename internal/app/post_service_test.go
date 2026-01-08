package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"blogcms/internal/domain"
)

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

type postRepoMock struct {
	created domain.Post
	tags    []string
}

func (m *postRepoMock) Create(_ context.Context, p *domain.Post) error {
	p.ID = 1
	m.created = *p
	return nil
}
func (m *postRepoMock) Update(_ context.Context, p *domain.Post) error { m.created = *p; return nil }
func (m *postRepoMock) BySlug(_ context.Context, _ string, _ bool) (domain.Post, error) {
	return domain.Post{}, domain.ErrNotFound
}
func (m *postRepoMock) List(_ context.Context, _ domain.PostListOptions) ([]domain.Post, error) {
	return nil, nil
}
func (m *postRepoMock) Delete(_ context.Context, _ int64) error { return nil }
func (m *postRepoMock) SetPostTags(_ context.Context, _ int64, tagNames []string) error {
	m.tags = tagNames
	return nil
}

type tagRepoMock struct{}

func (tagRepoMock) Cloud(_ context.Context) ([]domain.Tag, error) { return nil, nil }
func (tagRepoMock) Suggest(_ context.Context, _ string, _ int) ([]domain.Tag, error) { return nil, nil }

// Used by concurrency tests; avoids shared mutable state.
type postRepoNoop struct{ next atomic.Int64 }

func (m *postRepoNoop) Create(_ context.Context, p *domain.Post) error {
	p.ID = m.next.Add(1)
	return nil
}
func (m *postRepoNoop) Update(_ context.Context, _ *domain.Post) error { return nil }
func (m *postRepoNoop) BySlug(_ context.Context, _ string, _ bool) (domain.Post, error) {
	return domain.Post{}, domain.ErrNotFound
}
func (m *postRepoNoop) List(_ context.Context, _ domain.PostListOptions) ([]domain.Post, error) {
	return nil, nil
}
func (m *postRepoNoop) Delete(_ context.Context, _ int64) error { return nil }
func (m *postRepoNoop) SetPostTags(_ context.Context, _ int64, _ []string) error {
	return nil
}

func TestPostServiceCreate_RendersMarkdown_AndSetsTags(t *testing.T) {
	now := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)
	posts := &postRepoMock{}
	tags := tagRepoMock{}
	svc := NewPostService(posts, tags, fakeClock{t: now}, PostServiceOptions{MarkdownPoolSize: 2})

	p, err := svc.Create(context.Background(), CreatePostInput{
		Title:       "Hello World",
		ContentMD:   "# Title\n\nText.",
		TagsCSV:     "Go,  go, Postgres",
		IsPublished: true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if p.ID != 1 {
		t.Fatalf("expected id=1 got %d", p.ID)
	}
	if p.Slug != "hello-world" {
		t.Fatalf("expected slug hello-world got %q", p.Slug)
	}
	if p.ContentHTML == "" {
		t.Fatalf("expected rendered html")
	}
	if len(posts.tags) != 2 || posts.tags[0] != "go" || posts.tags[1] != "postgres" {
		t.Fatalf("expected normalized tags [go postgres], got %#v", posts.tags)
	}
	if p.PublishedAt == nil || !p.PublishedAt.Equal(now) {
		t.Fatalf("expected published_at=%v got %v", now, p.PublishedAt)
	}
}

func TestPostService_RenderMarkdown_IsSafeUnderConcurrentUse(t *testing.T) {
	now := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)
	posts := &postRepoNoop{}
	tags := tagRepoMock{}
	svc := NewPostService(posts, tags, fakeClock{t: now}, PostServiceOptions{MarkdownPoolSize: 2})

	const n = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := svc.Create(context.Background(), CreatePostInput{
				Title:       "Hello World",
				ContentMD:   "# T\n\nText.",
				TagsCSV:     "go",
				IsPublished: false,
			})
			errCh <- err
		}(i)
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
