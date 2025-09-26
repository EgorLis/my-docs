CREATE INDEX IF NOT EXISTS idx_docs_owner_updated
  ON mydocs.documents(owner_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_docs_public
  ON mydocs.documents(public) WHERE public = TRUE;

CREATE INDEX IF NOT EXISTS idx_docs_name_trgm
  ON mydocs.documents USING gin (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_shares_user
  ON mydocs.doc_shares(user_id);