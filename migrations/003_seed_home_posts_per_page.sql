-- Seed default home pagination settings for new installations.
-- Safe to run multiple times.

INSERT INTO settings (key, value) VALUES
  ('home.posts_per_page', '20')
ON CONFLICT (key) DO NOTHING;
