package cmsio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// Archive format version for compatibility checks.
	FormatVersion = 1
)

// Manifest describes the export bundle.
type Manifest struct {
	FormatVersion int       `json:"format_version"`
	ExportedAt    time.Time `json:"exported_at"`
	App           string    `json:"app"`
}

// Bundle contains DB data to export/import.
type Bundle struct {
	Manifest  Manifest   `json:"manifest"`
	Posts     []PostRow  `json:"posts"`
	Tags      []TagRow   `json:"tags"`
	PostTags  []PostTag  `json:"post_tags"`
	Settings  []Setting  `json:"settings,omitempty"`
	Checksums []Checksum `json:"checksums,omitempty"`
}

type PostRow struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	ContentMD   string     `json:"content_md"`
	ContentHTML string     `json:"content_html"`
	IsPublished bool       `json:"is_published"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	PublishedAt *time.Time `json:"published_at"`
}

type TagRow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type PostTag struct {
	PostID int64 `json:"post_id"`
	TagID  int64 `json:"tag_id"`
}

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Checksum struct {
	Path   string `json:"path"`   // relative to uploads/
	SHA256 string `json:"sha256"` // hex
	Size   int64  `json:"size"`
}

// ExportOptions controls what is included.
type ExportOptions struct {
	IncludeUploads  bool
	IncludeSettings bool
	// If true, export only files referenced from posts (best-effort, via uploadBasePrefix).
	// If false, export everything in uploadDir.
	UploadsReferencedOnly bool
	// Optional upload base prefix (e.g. "/uploads/") to detect referenced files in markdown/html.
	UploadBasePrefix string
}

// ImportOptions controls how data is restored.
type ImportOptions struct {
	TruncateTables   bool
	OverwriteUploads bool
	IncludeSettings  bool // if false, ignore settings from bundle even if present
}

// ExportToFile exports DB content and uploads into a gzipped tar archive at outPath.
func ExportToFile(ctx context.Context, db *sql.DB, uploadDir string, outPath string, opt ExportOptions) error {
	if strings.TrimSpace(outPath) == "" {
		return errors.New("outPath is required")
	}

	bundle, err := exportDB(ctx, db, opt.IncludeSettings)
	if err != nil {
		return err
	}
	bundle.Manifest = Manifest{
		FormatVersion: FormatVersion,
		ExportedAt:    time.Now().UTC(),
		App:           "blogcms",
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Write manifest and db bundle.
	if err := writeJSONEntry(tw, "manifest.json", bundle.Manifest); err != nil {
		return err
	}
	if err := writeJSONEntry(tw, "db/bundle.json", bundle); err != nil {
		return err
	}

	if !opt.IncludeUploads {
		return nil
	}

	// Uploads
	if strings.TrimSpace(uploadDir) == "" {
		return errors.New("uploads requested but uploadDir is empty")
	}
	uploadDir = filepath.Clean(uploadDir)

	var want map[string]struct{}
	if opt.UploadsReferencedOnly {
		want = referencedUploads(bundle, opt.UploadBasePrefix)
	}

	var checksums []Checksum
	err = filepath.WalkDir(uploadDir, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(uploadDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// skip dotfiles
		if strings.HasPrefix(filepath.Base(rel), ".") {
			return nil
		}
		if want != nil {
			if _, ok := want[rel]; !ok {
				return nil
			}
		}
		cs, err := addFile(tw, path, "uploads/"+rel)
		if err != nil {
			return err
		}
		checksums = append(checksums, cs)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk uploads: %w", err)
	}

	sort.Slice(checksums, func(i, j int) bool { return checksums[i].Path < checksums[j].Path })
	// Store checksums in a separate file for quick verification without parsing db bundle.
	if err := writeJSONEntry(tw, "uploads/checksums.json", checksums); err != nil {
		return err
	}

	return nil
}

func exportDB(ctx context.Context, db *sql.DB, includeSettings bool) (Bundle, error) {
	var out Bundle

	// Posts
	{
		rows, err := db.QueryContext(ctx, `
			SELECT id, title, slug, content_md, content_html, is_published, created_at, updated_at, published_at
			FROM posts
			ORDER BY id ASC`)
		if err != nil {
			return out, fmt.Errorf("query posts: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var p PostRow
			if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.ContentMD, &p.ContentHTML, &p.IsPublished, &p.CreatedAt, &p.UpdatedAt, &p.PublishedAt); err != nil {
				return out, fmt.Errorf("scan posts: %w", err)
			}
			out.Posts = append(out.Posts, p)
		}
		if err := rows.Err(); err != nil {
			return out, fmt.Errorf("iterate posts: %w", err)
		}
	}

	// Tags
	{
		rows, err := db.QueryContext(ctx, `SELECT id, name, slug FROM tags ORDER BY id ASC`)
		if err != nil {
			return out, fmt.Errorf("query tags: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var t TagRow
			if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
				return out, fmt.Errorf("scan tags: %w", err)
			}
			out.Tags = append(out.Tags, t)
		}
		if err := rows.Err(); err != nil {
			return out, fmt.Errorf("iterate tags: %w", err)
		}
	}

	// Post-tags
	{
		rows, err := db.QueryContext(ctx, `SELECT post_id, tag_id FROM post_tags ORDER BY post_id ASC, tag_id ASC`)
		if err != nil {
			return out, fmt.Errorf("query post_tags: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var pt PostTag
			if err := rows.Scan(&pt.PostID, &pt.TagID); err != nil {
				return out, fmt.Errorf("scan post_tags: %w", err)
			}
			out.PostTags = append(out.PostTags, pt)
		}
		if err := rows.Err(); err != nil {
			return out, fmt.Errorf("iterate post_tags: %w", err)
		}
	}

	if includeSettings {
		rows, err := db.QueryContext(ctx, `SELECT key, value FROM settings ORDER BY key ASC`)
		if err != nil {
			return out, fmt.Errorf("query settings: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var s Setting
			if err := rows.Scan(&s.Key, &s.Value); err != nil {
				return out, fmt.Errorf("scan settings: %w", err)
			}
			out.Settings = append(out.Settings, s)
		}
		if err := rows.Err(); err != nil {
			return out, fmt.Errorf("iterate settings: %w", err)
		}
	}
	return out, nil
}

// ImportFromFile restores DB content and uploads from a gzipped tar archive at inPath.
func ImportFromFile(ctx context.Context, db *sql.DB, uploadDir string, inPath string, opt ImportOptions) error {
	if strings.TrimSpace(inPath) == "" {
		return errors.New("inPath is required")
	}
	f, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	var bundle *Bundle
	var manifest *Manifest

	// For uploads: extract as we stream.
	if uploadDir != "" {
		uploadDir = filepath.Clean(uploadDir)
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		name := filepath.ToSlash(hdr.Name)

		switch {
		case name == "manifest.json":
			var m Manifest
			if err := json.NewDecoder(tr).Decode(&m); err != nil {
				return fmt.Errorf("decode manifest: %w", err)
			}
			manifest = &m

		case name == "db/bundle.json":
			var b Bundle
			if err := json.NewDecoder(tr).Decode(&b); err != nil {
				return fmt.Errorf("decode bundle: %w", err)
			}
			bundle = &b

		case strings.HasPrefix(name, "uploads/") && !strings.HasSuffix(name, "/") && opt.OverwriteUploads:
			if uploadDir == "" {
				// ignore
				if _, err := io.Copy(io.Discard, tr); err != nil {
					return err
				}
				continue
			}
			rel := strings.TrimPrefix(name, "uploads/")
			if rel == "" || strings.Contains(rel, "..") {
				return fmt.Errorf("unsafe upload path in archive: %q", name)
			}
			dst := filepath.Join(uploadDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
				return fmt.Errorf("mkdir uploads: %w", err)
			}
			if !opt.OverwriteUploads {
				if _, err := os.Stat(dst); err == nil {
					// skip existing
					if _, err := io.Copy(io.Discard, tr); err != nil {
						return err
					}
					continue
				}
			}
			tmp := dst + ".tmp"
			out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
			if err != nil {
				return fmt.Errorf("write upload: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("write upload: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close upload: %w", err)
			}
			if err := os.Rename(tmp, dst); err != nil {
				return fmt.Errorf("rename upload: %w", err)
			}

		default:
			// discard content
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return err
			}
		}
	}

	if manifest == nil {
		return errors.New("archive missing manifest.json")
	}
	if manifest.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported archive format_version=%d (expected %d)", manifest.FormatVersion, FormatVersion)
	}
	if bundle == nil {
		return errors.New("archive missing db/bundle.json")
	}
	if !opt.IncludeSettings {
		bundle.Settings = nil
	}

	return importDB(ctx, db, *bundle, opt.TruncateTables)
}

func importDB(ctx context.Context, db *sql.DB, b Bundle, truncate bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if truncate {
		// Order matters due to FK constraints.
		if _, err := tx.ExecContext(ctx, `TRUNCATE TABLE post_tags, posts, tags, settings RESTART IDENTITY CASCADE`); err != nil {
			return fmt.Errorf("truncate: %w", err)
		}
	}

	// Insert tags first.
	for _, t := range b.Tags {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tags (id, name, slug) VALUES ($1,$2,$3)
			ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, slug=EXCLUDED.slug`,
			t.ID, t.Name, t.Slug)
		if err != nil {
			return fmt.Errorf("insert tags: %w", err)
		}
	}

	// Posts
	for _, p := range b.Posts {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO posts (id, title, slug, content_md, content_html, is_published, created_at, updated_at, published_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO UPDATE SET
				title=EXCLUDED.title,
				slug=EXCLUDED.slug,
				content_md=EXCLUDED.content_md,
				content_html=EXCLUDED.content_html,
				is_published=EXCLUDED.is_published,
				created_at=EXCLUDED.created_at,
				updated_at=EXCLUDED.updated_at,
				published_at=EXCLUDED.published_at`,
			p.ID, p.Title, p.Slug, p.ContentMD, p.ContentHTML, p.IsPublished, p.CreatedAt, p.UpdatedAt, p.PublishedAt)
		if err != nil {
			return fmt.Errorf("insert posts: %w", err)
		}
	}

	// Post tags
	for _, pt := range b.PostTags {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO post_tags (post_id, tag_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING`,
			pt.PostID, pt.TagID)
		if err != nil {
			return fmt.Errorf("insert post_tags: %w", err)
		}
	}

	// Settings
	for _, s := range b.Settings {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO settings (key, value) VALUES ($1,$2)
			ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`,
			s.Key, s.Value)
		if err != nil {
			return fmt.Errorf("insert settings: %w", err)
		}
	}

	// Fix sequences
	if _, err := tx.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('posts','id'), COALESCE((SELECT MAX(id) FROM posts),0))`); err != nil {
		return fmt.Errorf("setval posts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('tags','id'), COALESCE((SELECT MAX(id) FROM tags),0))`); err != nil {
		return fmt.Errorf("setval tags: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func writeJSONEntry(tw *tar.Writer, name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	h := &tar.Header{
		Name:    filepath.ToSlash(name),
		Mode:    0o640,
		Size:    int64(len(b)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(h); err != nil {
		return fmt.Errorf("write header %s: %w", name, err)
	}
	if _, err := tw.Write(b); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func addFile(tw *tar.Writer, srcPath string, dstName string) (Checksum, error) {
	st, err := os.Stat(srcPath)
	if err != nil {
		return Checksum{}, fmt.Errorf("stat %s: %w", srcPath, err)
	}
	if !st.Mode().IsRegular() {
		return Checksum{}, nil
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return Checksum{}, fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer f.Close()

	h := &tar.Header{
		Name:    filepath.ToSlash(dstName),
		Mode:    0o640,
		Size:    st.Size(),
		ModTime: st.ModTime(),
	}
	if err := tw.WriteHeader(h); err != nil {
		return Checksum{}, fmt.Errorf("write header %s: %w", dstName, err)
	}

	hasher := sha256.New()
	mw := io.MultiWriter(tw, hasher)
	if _, err := io.Copy(mw, f); err != nil {
		return Checksum{}, fmt.Errorf("copy %s: %w", srcPath, err)
	}

	return Checksum{
		Path:   strings.TrimPrefix(filepath.ToSlash(dstName), "uploads/"),
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
		Size:   st.Size(),
	}, nil
}

// referencedUploads tries to find referenced files in post markdown/html.
// It only handles simple cases like "/uploads/<name>" or "uploads/<name>".
func referencedUploads(b Bundle, uploadBasePrefix string) map[string]struct{} {
	want := map[string]struct{}{}
	prefix := strings.TrimSpace(uploadBasePrefix)
	if prefix == "" {
		prefix = "/uploads/"
	}
	// normalize prefix to have leading and trailing slash like "/uploads/"
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	scan := func(s string) {
		// Fast substring scan rather than regex.
		for {
			i := strings.Index(s, prefix)
			if i < 0 {
				return
			}
			s = s[i+len(prefix):]
			// capture until first delimiter
			end := len(s)
			for j := 0; j < len(s); j++ {
				c := s[j]
				if c == '"' || c == '\'' || c == ')' || c == ']' || c == ' ' || c == '\n' || c == '\r' || c == '\t' {
					end = j
					break
				}
			}
			name := s[:end]
			name = strings.Trim(name, "/")
			if name != "" && !strings.Contains(name, "..") {
				// upload handler stores flat filenames, but keep path support.
				want[name] = struct{}{}
			}
			if end == len(s) {
				return
			}
			s = s[end:]
		}
	}

	for _, p := range b.Posts {
		scan(p.ContentMD)
		scan(p.ContentHTML)
	}
	return want
}
