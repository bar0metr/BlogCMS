# Testing strategy

This project is designed to be testable via interfaces (repositories, clock, session store).

## 1) Unit tests (fast, deterministic)
Purpose:
- Validate business logic in isolation (slugging, tag normalization, publish timestamps, markdown rendering).
- No network, no database.

Example:
- `internal/app/slug_test.go` checks `Slugify`.
- `internal/app/post_service_test.go` verifies `PostService.Create` sets slug, normalizes tags, sets `PublishedAt`.

Run:
```bash
go test ./internal/app -run Test
```

## 2) Integration tests (real dependencies)
Purpose:
- Validate SQL, schema compatibility, transaction boundaries.
- Catch issues that unit tests cannot (constraints, joins, default values).

Approach:
- Run a real Postgres (Docker is easiest).
- Provide `BLOGCMS_TEST_DSN` to point tests to a disposable DB.
- Tests apply migrations before exercising repositories.

Example:
- `internal/postgres/integration_test.go` (skips unless `BLOGCMS_TEST_DSN` is set).

Run:
```bash
docker compose up -d postgres

export BLOGCMS_TEST_DSN="postgres://blogcms:blogcms@localhost:5432/blogcms?sslmode=disable"
go test ./internal/postgres -run TestIntegration -count=1
```

Operational note:
- For strict isolation, create a dedicated test database (e.g. `blogcms_test`) and point DSN there.

## 3) HTTP handler tests (in-memory server)
Purpose:
- Validate routing, status codes, auth redirects, template rendering success paths.

Recommended pattern:
- Construct `web.Server` with fake repositories/services (mocks) and `httptest.NewServer`.
- Assert on status codes and body fragments.

Example skeleton:
```go
ts := httptest.NewServer(serverHandler)
defer ts.Close()

resp, _ := http.Get(ts.URL + "/")
if resp.StatusCode != http.StatusOK { ... }
```

## 4) End-to-end smoke tests (manual or CI script)
Purpose:
- Validate “it works” from the user perspective: create post -> publish -> view -> tag filter -> settings.

Recommended:
- A small shell script in CI:
  - start Postgres
  - apply migration
  - insert admin user
  - start server
  - `curl` the public endpoints and admin login flow

## 5) Static checks
Purpose:
- Prevent regression in style and common defects.

Recommended:
- `go test ./...`
- `go vet ./...`
- `staticcheck ./...` (optional)
- `golangci-lint run` (optional)


### Security regression tests

- `internal/web/middleware_test.go` contains unit tests for CSRF protection middleware.
