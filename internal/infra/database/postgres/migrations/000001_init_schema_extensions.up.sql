CREATE SCHEMA IF NOT EXISTS mydocs;

-- расширения ставим в БД (не в схему)
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;