# Setup (PostgreSQL + BlogCMS)

This document covers both local installation and a production-style deployment using systemd (without Docker).

## 0) Requirements

- Go 1.23+
- PostgreSQL 14+
- `psql`

## 1) Create DB + user (manual)

Run as a PostgreSQL superuser (often the `postgres` OS user):

```bash
sudo -u postgres psql
```

```sql
CREATE USER blogcms WITH PASSWORD 'CHANGE_ME_STRONG_PASSWORD';
CREATE DATABASE blogcms OWNER blogcms;
GRANT ALL PRIVILEGES ON DATABASE blogcms TO blogcms;
\q
```

## 2) Apply migrations

Set DSN (adjust credentials/host/port):

```bash
export BLOGCMS_DSN="postgres://blogcms:CHANGE_ME_STRONG_PASSWORD@localhost:5432/blogcms?sslmode=disable"
```

Apply schema + initial settings:

```bash
psql "$BLOGCMS_DSN" -f migrations/001_init.sql
psql "$BLOGCMS_DSN" -f migrations/002_seed_settings.sql
psql "$BLOGCMS_DSN" -f migrations/003_seed_home_posts_per_page.sql
```

Notes:
- `002_seed_settings.sql` is idempotent (`ON CONFLICT DO NOTHING`) and can be re-applied safely.
- On a new host/new database you apply **all** migrations in order.

## 3) Configure uploads directory

Uploads are stored on disk (not in the DB).

For local development you can keep the default:

- `./data/uploads`

For a server install, use a persistent directory (example):

- `/var/lib/blogcms/uploads`

Create it and set permissions for the service user:

```bash
sudo mkdir -p /var/lib/blogcms/uploads
sudo chown -R blogcms:blogcms /var/lib/blogcms
sudo chmod 0750 /var/lib/blogcms/uploads
```

## 4) Configure and run the server (local)

### Option A: YAML config (recommended)

Copy and edit:

```bash
cp configs/config.example.yaml configs/config.yaml
nano configs/config.yaml
```

Set a strong session key:

```bash
openssl rand -hex 32
```

Paste into:

```yaml
security:
  session_key: "..."
```

Run:

```bash
go run ./cmd/blogcms -config configs/config.yaml
```

### Option B: environment variables (no config file)

```bash
export BLOGCMS_ADDR=":8443"
export BLOGCMS_DSN="postgres://blogcms:CHANGE_ME_STRONG_PASSWORD@localhost:5432/blogcms?sslmode=disable"
export BLOGCMS_SESSION_KEY="$(openssl rand -hex 32)"
export BLOGCMS_COOKIE_SECURE="false"
export BLOGCMS_REQUEST_TIMEOUT="10s"
export BLOGCMS_DB_MAX_OPEN_CONNS="50"
export BLOGCMS_DB_MAX_IDLE_CONNS="25"
export BLOGCMS_DB_CONN_MAX_LIFETIME="30m"
export BLOGCMS_DB_CONN_MAX_IDLE_TIME="5m"
export BLOGCMS_DB_PING_TIMEOUT="5s"
export BLOGCMS_MD_POOL="4"
export BLOGCMS_SETTINGS_CACHE_TTL="30s"
export BLOGCMS_TAGCLOUD_CACHE_TTL="30s"
export BLOGCMS_UPLOAD_DIR="./data/uploads"
export BLOGCMS_UPLOAD_PUBLIC_BASE="/uploads/"
export BLOGCMS_MAX_UPLOAD_MB="10"

go run ./cmd/blogcms
```

## 5) Create/update the admin user

Use the helper tool:

```bash
go run ./cmd/addadmin -config configs/config.yaml -username admin
```

This prompts for the password (not echoed) and upserts the admin user.

## 6) Deployment with systemd (non-root)

This section is designed for AlmaLinux/RHEL-like systems, but works on any systemd Linux.

### 6.1) Create a dedicated OS user

```bash
sudo useradd --system --create-home --home-dir /var/lib/blogcms --shell /sbin/nologin blogcms
```

### 6.2) Install the application

Example layout:

- code: `/opt/blogcms`
- binaries: `/usr/local/bin/blogcms` and `/usr/local/bin/addadmin`
- config: `/etc/blogcms/config.yaml`
- uploads: `/var/lib/blogcms/uploads`

```bash
sudo mkdir -p /opt/blogcms /etc/blogcms /var/lib/blogcms/uploads
sudo chown -R blogcms:blogcms /var/lib/blogcms
sudo chmod 0750 /var/lib/blogcms /var/lib/blogcms/uploads
```

Copy the project to `/opt/blogcms` (git clone or rsync), then build:

```bash
cd /opt/blogcms
sudo -u blogcms go build -o /tmp/blogcms ./cmd/blogcms
sudo -u blogcms go build -o /tmp/addadmin ./cmd/addadmin
sudo mv /tmp/blogcms /usr/local/bin/blogcms
sudo mv /tmp/addadmin /usr/local/bin/addadmin
sudo chmod 0755 /usr/local/bin/blogcms /usr/local/bin/addadmin
```

Copy config:

```bash
sudo cp configs/config.example.yaml /etc/blogcms/config.yaml
sudo nano /etc/blogcms/config.yaml
```

Set:

- `db.dsn` (production credentials)
- `security.session_key` (strong random string)
- `security.cookie_secure: true` when running behind HTTPS
- `storage.upload_dir: /var/lib/blogcms/uploads`

Make config readable by the service user:

```bash
sudo chown root:blogcms /etc/blogcms/config.yaml
sudo chmod 0640 /etc/blogcms/config.yaml
```

### 6.3) Apply migrations (once per database)

Run from an admin workstation or on the host (does not require root):

```bash
export BLOGCMS_DSN="postgres://blogcms:CHANGE_ME@localhost:5432/blogcms?sslmode=disable"
psql "$BLOGCMS_DSN" -f /opt/blogcms/migrations/001_init.sql
psql "$BLOGCMS_DSN" -f /opt/blogcms/migrations/002_seed_settings.sql
psql "$BLOGCMS_DSN" -f /opt/blogcms/migrations/003_seed_home_posts_per_page.sql
```

### 6.4) Create the admin user (once)

```bash
sudo -u blogcms /usr/local/bin/addadmin -config /etc/blogcms/config.yaml -username admin
```

### 6.5) Create a systemd unit

Create `/etc/systemd/system/blogcms.service`:

```ini
[Unit]
Description=BlogCMS
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=blogcms
Group=blogcms
ExecStart=/usr/local/bin/blogcms -config /etc/blogcms/config.yaml
WorkingDirectory=/var/lib/blogcms
Restart=on-failure
RestartSec=2
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/blogcms
AmbientCapabilities=
CapabilityBoundingSet=

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now blogcms
sudo journalctl -u blogcms -f
```

### 6.6) Reverse proxy and HTTPS

For production, run behind an HTTPS reverse proxy (nginx/caddy/traefik) and set:

```yaml
security:
  cookie_secure: true
```

