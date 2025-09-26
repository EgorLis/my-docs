CREATE TABLE IF NOT EXISTS mydocs.doc_json (
  doc_id  UUID PRIMARY KEY REFERENCES mydocs.documents(id) ON DELETE CASCADE,
  body    JSONB NOT NULL
);