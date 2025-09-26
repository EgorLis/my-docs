CREATE TABLE IF NOT EXISTS mydocs.documents (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id       UUID NOT NULL REFERENCES mydocs.users(id) ON DELETE CASCADE,
  name           TEXT NOT NULL,
  mime_type      TEXT NOT NULL,
  file           BOOLEAN NOT NULL DEFAULT TRUE,
  public         BOOLEAN NOT NULL DEFAULT FALSE,
  size_bytes     BIGINT NOT NULL DEFAULT 0,
  storage_key    TEXT NOT NULL,
  content_sha256 BYTEA NOT NULL,
  version        BIGINT NOT NULL DEFAULT 1,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);