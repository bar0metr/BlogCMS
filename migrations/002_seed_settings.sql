-- Seed default UI settings for new installations.
-- Safe to run multiple times.

INSERT INTO settings (key, value) VALUES
  ('blog.title', 'My Blog'),
  ('blog.about', 'Go + PostgreSQL. HTML templates. Minimal dependencies'),
  ('blog.footer', '')
ON CONFLICT (key) DO NOTHING;
