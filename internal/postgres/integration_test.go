package postgres

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blogcms/internal/domain"
)

func TestIntegration_PostLifecycle(t *testing.T) {
	dsn := os.Getenv("BLOGCMS_TEST_DSN")
	if dsn == "" {
		t.Skip("set BLOGCMS_TEST_DSN to run integration tests (e.g. postgres://blogcms:blogcms@localhost:5432/blogcms?sslmode=disable)")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	migPath := filepath.Join("..", "..", "migrations", "001_init.sql")
	if err := applySQLFile(ctx, db, migPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	posts := NewPostRepository(db)
	tags := NewTagRepository(db)

	now := time.Now().UTC()
	p := domain.Post{
		Title:       "Integration Post",
		Slug:        "integration-post-" + time.Now().Format("150405"),
		ContentMD:   "Hi",
		ContentHTML: "<p>Hi</p>\\n",
		IsPublished: true,
		CreatedAt:   now,
		UpdatedAt:   now,
		PublishedAt: &now,
	}

	if err := posts.Create(ctx, &p); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := posts.SetPostTags(ctx, p.ID, []string{"go", "postgres"}); err != nil {
		t.Fatalf("set tags: %v", err)
	}

	got, err := posts.BySlug(ctx, p.Slug, true)
	if err != nil {
		t.Fatalf("by slug: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("expected id %d got %d", p.ID, got.ID)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("expected 2 tags got %d", len(got.Tags))
	}

	cloud, err := tags.Cloud(ctx)
	if err != nil {
		t.Fatalf("tag cloud: %v", err)
	}
	if len(cloud) == 0 {
		t.Fatalf("expected non-empty tag cloud")
	}
}

func applySQLFile(ctx context.Context, db *sql.DB, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(b))
	return err
}
