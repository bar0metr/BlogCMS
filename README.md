# BlogCMS

A minimal personal blog CMS written in Go (Go + PostgreSQL + HTML templates), with an admin UI, tag cloud, Markdown editor, CSRF protection, and uploads.

> Also, my personal blog runs on it: https://grub-loader.ru
> 
## Features

- Posts (Markdown) with tags
- Public pages with a persistent tag cloud sidebar
- Admin area (`/admin/login`) with:
  - Create/edit posts (draft/publish)
  - Settings page (stored in DB): blog title, “About”, footer
  - Upload images/files directly from the editor
- Security:
  - Passwords: bcrypt
  - Sessions: signed cookies + server-side session store
  - CSRF protection on all state-changing admin actions (including uploads)
  - Login rate limiting
  - Security headers (CSP, nosniff, frame deny, etc.)

## Requirements

- Go 1.23+
- PostgreSQL 14+
- `psql`

## Documentation

- Installation and deployment (including systemd): `docs/SETUP.md`
- Testing: `docs/TESTING.md`

## Quick start (local)

Follow `docs/SETUP.md`.

Endpoints:
- Public: `http://127.0.0.1:8443/`
- Admin login: `http://127.0.0.1:8443/admin/login`

## Configuration
### Production scaling knobs

Key defaults are set for defensive behavior, but you are expected to tune them for your host:

- `server.request_timeout`: bounds handler execution time (also bounds DB calls via request context).
- `db.max_open_conns` / `db.max_idle_conns`: database pool sizing.
- `app.markdown_renderer_pool`: bounds concurrent Markdown->HTML conversions (CPU backpressure).
- `app.settings_cache_ttl` / `app.tagcloud_cache_ttl`: reduces DB load on hot pages.


Config file: `configs/config.yaml` (example: `configs/config.example.yaml`)

### Uploads

Uploads are stored on disk (not in the DB).

- `storage.upload_dir`: directory where files are written
- `storage.public_base_url`: URL prefix used to serve files (default `/uploads/`)
- `storage.max_upload_mb`: request size limit (server-side enforced)

Example:

```yaml
storage:
  upload_dir: "/var/lib/blogcms/uploads"
  public_base_url: "/uploads/"
  max_upload_mb: 10
```

The server serves files from `storage.upload_dir` under `storage.public_base_url`.

Security notes:
- SVG uploads are rejected (XSS risk).
- Only a small allowlist of content-types is accepted by default.

## Build

```bash
go mod tidy
go test ./...
go build -o ./bin/blogcms ./cmd/blogcms
```

Utility to create/update admin user:

```bash
go build -o ./bin/addadmin ./cmd/addadmin
```


## Backup / Restore (import/export)

BlogCMS ships with a CLI utility that exports posts (with tags and optional settings) together with uploaded files, and imports the archive on a clean installation.

See `docs/IMPORT_EXPORT.md` for usage and flags.

## Testing

Run all unit tests:

```bash
go test ./...
```

Integration tests (PostgreSQL) require a dedicated test DSN:

```bash
export BLOGCMS_TEST_DSN="postgres://blogcms:CHANGE_ME@localhost:5432/blogcms_test?sslmode=disable"
go test ./... -run Integration
```

See `docs/TESTING.md` for details.

