package cmsio

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestIntegration_ExportImportRoundtrip(t *testing.T) {
	dsn := os.Getenv("BLOGCMS_TEST_DSN")
	if dsn == "" {
		t.Skip("set BLOGCMS_TEST_DSN to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Ensure schema exists (apply migrations if the test DB is empty).
	if err := ensureSchema(ctx, db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// Clean.
	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE post_tags, posts, tags, settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Seed minimal settings and content.
	if _, err := db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES ('blog_title','Test') ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	var tagID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO tags (name, slug) VALUES ('Go','go') RETURNING id`).Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	uploadDir1 := t.TempDir()
	uploadName := "abc123.png"
	uploadPath := filepath.Join(uploadDir1, uploadName)
	if err := os.WriteFile(uploadPath, []byte("fakepng"), 0o640); err != nil {
		t.Fatalf("write upload: %v", err)
	}

	var postID int64
	md := "Hello ![](/uploads/" + uploadName + ")"
	html := "<p>Hello <img src=\"/uploads/" + uploadName + "\" /></p>"
	if err := db.QueryRowContext(ctx, `
		INSERT INTO posts (title, slug, content_md, content_html, is_published, created_at, updated_at, published_at)
		VALUES ('Title','title', $1, $2, true, now(), now(), now())
		RETURNING id`, md, html).Scan(&postID); err != nil {
		t.Fatalf("seed post: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO post_tags (post_id, tag_id) VALUES ($1,$2)`, postID, tagID); err != nil {
		t.Fatalf("seed post_tags: %v", err)
	}

	archive := filepath.Join(t.TempDir(), "export.tar.gz")
	if err := ExportToFile(ctx, db, uploadDir1, archive, ExportOptions{
		IncludeUploads:        true,
		IncludeSettings:       true,
		UploadsReferencedOnly: true,
		UploadBasePrefix:      "/uploads/",
	}); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Import into clean DB + new uploads dir.
	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE post_tags, posts, tags, settings RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate2: %v", err)
	}
	uploadDir2 := t.TempDir()

	if err := ImportFromFile(ctx, db, uploadDir2, archive, ImportOptions{
		TruncateTables:   false, // already truncated
		OverwriteUploads: true,
		IncludeSettings:  true,
	}); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Verify restored post and tag relation.
	var gotPosts int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM posts`).Scan(&gotPosts); err != nil {
		t.Fatalf("count posts: %v", err)
	}
	if gotPosts != 1 {
		t.Fatalf("expected 1 post, got %d", gotPosts)
	}
	var gotTags int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tags`).Scan(&gotTags); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if gotTags != 1 {
		t.Fatalf("expected 1 tag, got %d", gotTags)
	}
	var gotPT int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM post_tags`).Scan(&gotPT); err != nil {
		t.Fatalf("count post_tags: %v", err)
	}
	if gotPT != 1 {
		t.Fatalf("expected 1 post_tag, got %d", gotPT)
	}

	// Verify upload file restored.
	if b, err := os.ReadFile(filepath.Join(uploadDir2, uploadName)); err != nil {
		t.Fatalf("read restored upload: %v", err)
	} else if string(b) != "fakepng" {
		t.Fatalf("restored upload content mismatch")
	}

	// Verify settings restored.
	var v string
	if err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='blog_title'`).Scan(&v); err != nil {
		t.Fatalf("settings restored: %v", err)
	}
	if v != "Test" {
		t.Fatalf("settings restored mismatch: %q", v)
	}
}

func ensureSchema(ctx context.Context, db *sql.DB) error {
	// Check if posts table exists.
	var exists bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM information_schema.tables
		  WHERE table_schema='public' AND table_name='posts'
		)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	// Apply migrations from project tree if available; otherwise fail.
	// We look for ./migrations relative to current working dir.
	cwd, _ := os.Getwd()
	migDir := filepath.Join(cwd, "migrations")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		return err
	}
	// apply in lexical order
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(migDir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(b)); err != nil {
			return err
		}
	}
	return nil
}
