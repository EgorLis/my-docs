CREATE TABLE IF NOT EXISTS mydocs.doc_shares (
  doc_id   UUID NOT NULL REFERENCES mydocs.documents(id) ON DELETE CASCADE,
  user_id  UUID NOT NULL REFERENCES mydocs.users(id) ON DELETE CASCADE,
  can_read BOOLEAN NOT NULL DEFAULT TRUE,
  PRIMARY KEY (doc_id, user_id)
);