CREATE TABLE IF NOT EXISTS mydocs.users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  login       TEXT NOT NULL UNIQUE,
  pass_hash   BYTEA NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);