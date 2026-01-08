package version

var (
	Version = "1.0.4"
)

/*
Changelog:

============================
1.0.4 - Home pagination
============================

- Added: home page pagination controls rendered below latest posts.
- Added: configurable home page page size (posts per page) exposed in Admin -> Settings.
- Added: idempotent migration to seed the new setting for new installs.

=========================
1.0.3 - Styling cleanup
=========================

- Changed: moved all template-embedded and inline styles into an external /static/site.css served from embedded assets.
- Improved: adjusted home page post preview fade so the first line remains fully readable, with a gentle fade only on subsequent lines.

=========================
1.0.2 - Tags, slugs, and UX fixes
=========================

- Fixed: corrected slug generation logic to preserve historical behavior for empty and non-alphanumeric-only inputs, restoring test invariants while remaining Unicode-safe.
- Fixed: eliminated tag corruption caused by slug collisions when using non-ASCII tag names; tags are no longer implicitly merged under a shared fallback slug.
- Fixed: aligned all test doubles and fakes with updated domain interfaces after introducing tag suggestion capabilities.
- Added: tag suggestion endpoint for admin UI, enabling prefix-based lookup of existing tags to reduce duplication and typos.
- Added: interactive tag editor in admin post form with YouTube-style “chip” UI, supporting autocomplete, deduplication, and keyboard-friendly editing.
- Added: automatic post preview (excerpt) generation for the home page by stripping markup and extracting a concise text summary.
- Improved: home page UX by displaying post previews under titles with a subtle visual fade, improving scanability without full-content rendering.
- Refactor: isolated excerpt generation into a reusable helper to keep handlers and templates simpler.
- Tests: ensured full test suite compatibility after slug and tag logic changes; no regressions in existing coverage.

=========================
1.0.1 - Stability hotfixes
=========================

- Fixed: resolved intermittent request timeouts under load by tightening request-scoped context deadlines and aligning them with server write timeouts.
- Fixed: prevented cache stampede on hot pages by adding singleflight-style suppression for concurrent cache misses.
- Fixed: eliminated a rare deadlock scenario in background janitors during shutdown by ensuring cancellation order is deterministic.
- Fixed: corrected admin settings cache invalidation so UI changes propagate immediately to public pages.
- Fixed: improved shutdown behavior to avoid truncating in-flight responses when systemd stops the service.
- Improved: hardened error propagation from parallelized view-data gathering; first error cancels remaining work and returns consistent HTTP status.
- Improved: reduced allocation churn in markdown rendering pipeline under high concurrency.
- Docs: clarified production tuning defaults and introduced a “recommended baseline” for high-traffic deployments.

===============================
1.0.0 - Production-ready release
===============================

- Added: fully configurable DB connection pooling (max open/idle conns, lifetimes, ping timeout) via config/env.
- Added: HTTP server hardening knobs (ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout, MaxHeaderBytes) configurable via config/env.
- Added: request timeout middleware to bound worst-case latency and prevent resource exhaustion.
- Added: bounded markdown renderer pool to provide backpressure and remove global serialization bottlenecks.
- Added: TTL caches for hot settings-derived view fragments (title/about/footer/tag cloud) to cut database read amplification.
- Added: singleflight-style cache fill to prevent thundering herd on cache expiry.
- Added: deterministic cache invalidation on admin settings changes.
- Added: improved observability hooks and more consistent error messages for operational triage.
- Changed: refactored postgres open/init to accept explicit options instead of implicit defaults.
- Changed: reworked server bootstrap to start/stop background janitors under a shared cancellation context.
- Security: tightened default timeouts to reduce exposure to slowloris-style header attacks.
- Docs: moved all installation/deployment procedures into SETUP, leaving README as an overview with pointers.

====================================
0.9.0 - Scaling and performance sprint
====================================

- Added: configuration schema expansion (server timeouts, request timeout, DB pool sizing, cache TTLs, renderer pool size).
- Added: env overrides for all production-critical knobs (BLOGCMS_*), maintaining YAML as the canonical baseline.
- Added: TTL cache utility with concurrency safety to reduce repeated reads for frequently requested fragments.
- Improved: parallel rendering of independent view components to reduce tail latency on the home page.
- Improved: reduced DB round-trips on hot endpoints through caching and query consolidation.
- Improved: made background cleanup intervals configurable to balance memory footprint vs CPU overhead.
- Changed: tightened handler context usage so DB operations reliably terminate on client disconnect.
- Tests: added race-oriented tests around concurrent post creation and cache fill behavior.

=================================
0.8.0 - Concurrency and resilience
=================================

- Added: background janitors for in-memory stores (sessions, login limiter) to prevent unbounded growth in long-running processes.
- Added: structured cancellation for internal parallel work; failures propagate quickly and cancel siblings.
- Fixed: removed several “best-effort” error paths that could silently drop failures under concurrency.
- Improved: more deterministic shutdown path; background work stops before closing shared resources.
- Tests: introduced cleanup verification tests for expiry-based in-memory stores.

==============================
0.7.0 - Admin and ops hardening
==============================

- Added: admin settings workflow stabilization and safer persistence semantics.
- Added: clearer separation of concerns between web handlers and app services.
- Improved: error handling on admin endpoints to avoid partial updates.
- Improved: refined systemd unit defaults (restart policy, limits, working directory).
- Docs: expanded SETUP with reverse proxy notes and service management guidance.

=============================
0.6.0 - Content pipeline update
=============================

- Added: markdown rendering improvements and safer HTML generation defaults.
- Improved: post creation/update logic with better validation and clearer errors.
- Improved: reduced template duplication and centralized view helper functions.
- Fixed: edge cases around empty content fields and malformed slugs.
- Tests: extended coverage around post lifecycle (create/update/list).

===============================
0.5.0 - Authentication and access
===============================

- Added: session store and login rate limiting (basic abuse protection).
- Added: admin authentication guards for protected endpoints.
- Improved: cookie/session handling and expiration behavior.
- Fixed: inconsistent redirect flows after login/logout.
- Tests: added unit tests for session operations and limiter behavior.

=============================
0.4.0 - Storage and migrations
=============================

- Added: Postgres persistence layer with schema initialization/migration baseline.
- Added: repository interfaces to decouple app layer from DB implementation.
- Improved: transactional updates where appropriate to avoid partial writes.
- Fixed: several SQL query edge cases and ordering issues.
- Docs: documented DB setup and basic migration workflow.

=========================
0.3.0 - Web UI and routing
=========================

- Added: public pages (home, post page, tags, about) and initial HTML templates.
- Added: routing and handler structure with clear separation for admin/public.
- Improved: common view data computation and template rendering consistency.
- Fixed: template rendering errors and missing assets.
- Tests: introduced handler-level unit tests for core routes.

============================
0.2.0 - Project architecture
============================

- Added: initial module layout (cmd/, internal/) and application/service boundaries.
- Added: config loading with sensible defaults and example config.
- Added: basic logging and error wrapping conventions.
- Improved: test harness scaffolding and CI-friendly commands.
- Docs: introduced README overview and initial SETUP instructions.

=====================
0.0.1 - Initial setup
=====================

- Added: initial Go module setup and baseline repository structure.
- Added: minimal HTTP server scaffold and placeholders for routes/templates.
- Added: starter documentation and example configuration file.
- Added: initial unit test skeletons and local dev instructions.

*/
