# Import / Export (full backup)

BlogCMS includes a CLI utility that exports **all posts** (with tags and optional settings) together with uploaded files and restores them on a clean installation.

The utility uses the same YAML config as the server (`db.*` and `storage.upload_dir`).

## Build

```bash
go build -o bin/cmsio ./cmd/cmsio
```

## Export

Export database content + uploads into a single `tar.gz` archive:

```bash
./bin/cmsio export -config ./configs/config.yaml -out ./blogcms-backup.tar.gz
```

Options:

- `--no-uploads` — export DB only
- `--no-settings` — do not export the `settings` table
- `--uploads-referenced-only` — include only files referenced in post content (best-effort)
- `--upload-base "/uploads/"` — override the URL prefix used to detect referenced files

Archive layout:

- `manifest.json`
- `db/bundle.json` (posts, tags, post_tags, optionally settings)
- `uploads/<files...>`
- `uploads/checksums.json` (sha256 + size for exported files)

## Import

Important prerequisites:

1. You must create the target database and apply migrations first (BlogCMS does not auto-migrate).
2. `storage.upload_dir` in the config must point to the desired uploads directory on the target system.

Import an archive:

```bash
./bin/cmsio import -config ./configs/config.yaml -in ./blogcms-backup.tar.gz
```

Defaults:

- The utility **truncates** `post_tags`, `posts`, `tags`, `settings` before import.
- Upload files are **overwritten** if they already exist.

Options:

- `--no-truncate` — do not truncate before import (useful for merging with an existing DB)
- `--no-overwrite-uploads` — skip upload files that already exist
- `--no-settings` — ignore settings from the archive

## Compatibility / guarantees

- The import uses explicit IDs and then resets Postgres sequences to `MAX(id)` for `posts` and `tags`.
- Archive format version is validated (`manifest.format_version`).
- Import expects the schema to match the `migrations/` provided by this repository.
