package postgres

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/EgorLis/my-docs/internal/domain"
	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// ---- Postgres репозиторий (pgxpool) + golang-migrate ----

type PGRepo struct {
	logger *log.Logger
	pool   *pgxpool.Pool
	schema string
}

func NewPGRepo(ctx context.Context, logger *log.Logger, dsn, schema string) (*PGRepo, error) {
	// Запускаем golang-migrate используя pgx/stdlib
	if err := runMigrations(dsn, logger); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	// Создаем pgxpool
	logger.Println("initializing pgxpool...")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	logger.Println("pgxpool initialized")

	r := &PGRepo{pool: pool, schema: schema, logger: logger}
	return r, nil
}

func (r *PGRepo) Close() {
	r.logger.Println("closing pgxpool...")
	r.pool.Close()
	r.logger.Println("pgxpool closed")
}

// ---- Миграции через golang-migrate ----

//go:embed migrations/*.sql
var EmbeddedMigrations embed.FS

func runMigrations(dsn string, logger *log.Logger) error {
	// Открываем *sql.DB с помощью pgx stdlib. Важно: это отдельный экземпляр от pgxpool.
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("sql.Open pgx: %w", err)
	}
	defer sqldb.Close()

	driver, err := postgres.WithInstance(sqldb, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("postgres driver: %w", err)
	}

	src, err := iofs.New(EmbeddedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	defer m.Close()

	logger.Println("applying migrations...")
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Println("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}
	logger.Println("migrations applied successfully")
	return nil
}

// ---- Реализация репозитория ----

func (r *PGRepo) Ping(ctx context.Context) error {
	r.logger.Println("pinging database...")
	if err := r.pool.Ping(ctx); err != nil {
		r.logger.Printf("ping failed: %v", err)
		return err
	}
	r.logger.Println("ping successful")
	return nil
}

// Вспомогательно: билдер с плейсхолдерами $1, $2, ...
func (r *PGRepo) qb() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}

// Create implements domain.UsersRepo.
func (r *PGRepo) CreateUser(ctx context.Context, login string, passHash []byte) (domain.User, error) {
	q := r.qb().Insert(fmt.Sprintf("%s.users", r.schema)).
		Columns("login", "pass_hash").
		Values(login, passHash).
		Suffix("RETURNING id, login, pass_hash, created_at")

	sqlStr, args, _ := q.ToSql()
	row := r.pool.QueryRow(ctx, sqlStr, args...)

	var u domain.User
	if err := row.Scan(&u.ID, &u.Login, &u.PassHash, &u.CreatedAt); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// ByLogin implements domain.UsersRepo.
func (r *PGRepo) UserByLogin(ctx context.Context, login string) (domain.User, error) {
	q := r.qb().Select("id", "login", "pass_hash", "created_at").
		From(fmt.Sprintf("%s.users", r.schema)).
		Where(sq.Eq{"login": login})

	sqlStr, args, _ := q.ToSql()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Login, &u.PassHash, &u.CreatedAt); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// ByID implements domain.UsersRepo.
func (r *PGRepo) UserByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	q := r.qb().Select("id", "login", "pass_hash", "created_at").
		From(fmt.Sprintf("%s.users", r.schema)).
		Where(sq.Eq{"id": id})

	sqlStr, args, _ := q.ToSql()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Login, &u.PassHash, &u.CreatedAt); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (r *PGRepo) CreateDoc(ctx context.Context, meta domain.Document, jsonBody domain.DocJSON) (domain.Document, error) {
	// вставляем метаданные
	q := r.qb().Insert(fmt.Sprintf("%s.documents", r.schema)).
		Columns("owner_id", "name", "mime_type", "file", "public", "size_bytes", "storage_key", "content_sha256").
		Values(meta.OwnerID, meta.Name, meta.MIME, meta.File, meta.Public, meta.SizeBytes, meta.StorageKey, meta.SHA256).
		Suffix("RETURNING id, owner_id, name, mime_type, file, public, size_bytes, storage_key, content_sha256, version, created_at, updated_at")

	sqlStr, args, _ := q.ToSql()
	row := r.pool.QueryRow(ctx, sqlStr, args...)

	var out domain.Document
	if err := row.Scan(
		&out.ID, &out.OwnerID, &out.Name, &out.MIME, &out.File, &out.Public,
		&out.SizeBytes, &out.StorageKey, &out.SHA256, &out.Version, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return domain.Document{}, err
	}

	// json (опционально)
	if jsonBody != nil {
		payload, err := json.Marshal(jsonBody)
		if err != nil {
			return domain.Document{}, err
		}
		qj := r.qb().Insert(fmt.Sprintf("%s.doc_json", r.schema)).
			Columns("doc_id", "body").Values(out.ID, payload)
		sqlStr, args, _ = qj.ToSql()
		if _, err := r.pool.Exec(ctx, sqlStr, args...); err != nil {
			return domain.Document{}, err
		}
	}
	return out, nil
}

// Если forUser != nil, применяем ACL: владелец ИЛИ public ИЛИ есть share(can_read).
func (r *PGRepo) DocByID(ctx context.Context, id domain.DocID, forUser *domain.User) (domain.Document, domain.DocJSON, error) {
	docs := fmt.Sprintf("%s.documents d", r.schema)
	sb := r.qb().Select(
		"d.id", "d.owner_id", "d.name", "d.mime_type", "d.file", "d.public",
		"d.size_bytes", "d.storage_key", "d.content_sha256",
		"d.version", "d.created_at", "d.updated_at",
	).From(docs).Where(sq.Eq{"d.id": id})

	if forUser != nil {
		shares := fmt.Sprintf("%s.doc_shares s", r.schema)
		acl := sq.Or{
			sq.Eq{"d.owner_id": forUser.ID},
			sq.Eq{"d.public": true},
			sq.Expr("EXISTS (SELECT 1 FROM "+shares+" WHERE s.doc_id = d.id AND s.user_id = ? AND s.can_read = TRUE)", forUser.ID),
		}
		sb = sb.Where(acl)
	}

	sqlStr, args, _ := sb.ToSql()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var d domain.Document
	if err := row.Scan(
		&d.ID, &d.OwnerID, &d.Name, &d.MIME, &d.File, &d.Public,
		&d.SizeBytes, &d.StorageKey, &d.SHA256,
		&d.Version, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return domain.Document{}, nil, err
	}

	// doc_json (может отсутствовать)
	var jsonRaw []byte
	qj := r.qb().Select("body").
		From(fmt.Sprintf("%s.doc_json", r.schema)).
		Where(sq.Eq{"doc_id": id})
	sqlStr, args, _ = qj.ToSql()
	_ = r.pool.QueryRow(ctx, sqlStr, args...).Scan(&jsonRaw)

	var dj domain.DocJSON
	if len(jsonRaw) > 0 {
		_ = json.Unmarshal(jsonRaw, &dj)
	}

	return d, dj, nil
}

func (r *PGRepo) DocDelete(ctx context.Context, id domain.DocID, owner domain.UserID) error {
	q := r.qb().Delete(fmt.Sprintf("%s.documents", r.schema)).
		Where(sq.And{sq.Eq{"id": id}, sq.Eq{"owner_id": owner}})
	sqlStr, args, _ := q.ToSql()
	tag, err := r.pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sqlNoRowsErr("document not found or not owner")
	}
	return nil
}

func sqlNoRowsErr(msg string) error { return fmt.Errorf(msg) }

// Выдаёт документы пользователя (свои + публичные + расшаренные),
// если Login указан — эквивалент «покажи его документы, к которым у меня есть доступ/публичные».
// List implements domain.DocsRepo.
func (r *PGRepo) DocsList(ctx context.Context, me domain.User, f domain.ListFilter) ([]domain.Document, error) {
	docs := fmt.Sprintf("%s.documents d", r.schema)
	users := fmt.Sprintf("%s.users u", r.schema)
	shares := fmt.Sprintf("%s.doc_shares s", r.schema)

	sb := r.qb().Select(
		"d.id", "d.owner_id", "d.name", "d.mime_type", "d.file", "d.public",
		"d.size_bytes", "d.storage_key", "d.content_sha256",
		"d.version", "d.created_at", "d.updated_at",
	).From(docs).
		Join(users + " ON u.id = d.owner_id")

	// видимость относительно me
	visible := sq.Or{
		sq.Eq{"d.owner_id": me.ID},
		sq.Eq{"d.public": true},
		sq.Expr("EXISTS (SELECT 1 FROM "+shares+" WHERE s.doc_id = d.id AND s.user_id = ? AND s.can_read = TRUE)", me.ID),
	}
	sb = sb.Where(visible)

	// если задан login — показываем только документы этого пользователя (в рамках видимости)
	if f.Login != "" {
		sb = sb.Where(sq.Eq{"u.login": f.Login})
	}

	// фильтры key/value (белый список)
	switch f.Key {
	case "name":
		sb = sb.Where(sq.Eq{"d.name": f.Value})
	case "mime":
		sb = sb.Where(sq.Eq{"d.mime_type": f.Value})
	case "":
	default:
		// неизвестный ключ — игнор/ошибка (на твой выбор)
	}

	// сортировка по имени/дате создания
	switch f.Sort {
	case domain.SortByNameAsc:
		sb = sb.OrderBy("d.name ASC", "d.created_at DESC")
	case domain.SortByNameDesc:
		sb = sb.OrderBy("d.name DESC", "d.created_at DESC")
	case domain.SortByCreatedAsc:
		sb = sb.OrderBy("d.created_at ASC", "d.name ASC")
	case domain.SortByCreatedDesc, "":
		sb = sb.OrderBy("d.created_at DESC", "d.name ASC")
	}

	// кейсет-пагинация (опционально — если заполнены поля)
	if !f.AfterCreated.IsZero() || f.AfterID != uuid.Nil {
		// упрощённо: по created_at DESC, затем id
		sb = sb.Where(
			sq.Or{
				sq.Lt{"d.created_at": f.AfterCreated},
				sq.And{
					sq.Eq{"d.created_at": f.AfterCreated},
					sq.Lt{"d.id": f.AfterID},
				},
			},
		)
	}

	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	sb = sb.Limit(uint64(limit))

	sqlStr, args, _ := sb.ToSql()
	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.Document
	for rows.Next() {
		var d domain.Document
		if err := rows.Scan(
			&d.ID, &d.OwnerID, &d.Name, &d.MIME, &d.File, &d.Public,
			&d.SizeBytes, &d.StorageKey, &d.SHA256,
			&d.Version, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, d)
	}
	return res, rows.Err()
}

// Touch: увеличить версию и обновить updated_at — пригодится для ETag-кеша
func (r *PGRepo) Touch(ctx context.Context, id domain.DocID) error {
	q := r.qb().Update(fmt.Sprintf("%s.documents", r.schema)).
		SetMap(map[string]any{
			"version":    sq.Expr("version + 1"),
			"updated_at": sq.Expr("now()"),
		}).
		Where(sq.Eq{"id": id})
	sqlStr, args, _ := q.ToSql()
	_, err := r.pool.Exec(ctx, sqlStr, args...)
	return err
}

// ---------- SHARES ----------

// Вставляет/обновляет грант чтения по логину пользователя (находит user_id по login).
func (r *PGRepo) UpsertReadGrant(ctx context.Context, docID domain.DocID, login string, canRead bool) error {
	sub := r.qb().Select().
		Column("? AS doc_id", docID).
		Column("u.id AS user_id").
		Column("? AS can_read", canRead).
		From(fmt.Sprintf("%s.users u", r.schema)).
		Where(sq.Eq{"u.login": login})

	q := r.qb().Insert(fmt.Sprintf("%s.doc_shares", r.schema)).
		Columns("doc_id", "user_id", "can_read").
		Select(sub).
		Suffix("ON CONFLICT (doc_id, user_id) DO UPDATE SET can_read = EXCLUDED.can_read")

	sqlStr, args, _ := q.ToSql()
	_, err := r.pool.Exec(ctx, sqlStr, args...)
	return err
}

func (r *PGRepo) RemoveGrant(ctx context.Context, docID domain.DocID, login string) error {
	q := r.qb().Delete(fmt.Sprintf("%s.doc_shares", r.schema)).
		Where(sq.And{
			sq.Eq{"doc_id": docID},
			sq.Expr("user_id = (SELECT id FROM "+r.schema+".users WHERE login = ?)", login),
		})
	sqlStr, args, _ := q.ToSql()
	_, err := r.pool.Exec(ctx, sqlStr, args...)
	return err
}

func (r *PGRepo) ListGrantedLogins(ctx context.Context, docID domain.DocID) ([]string, error) {
	q := r.qb().Select("u.login").
		From(fmt.Sprintf("%s.doc_shares s", r.schema)).
		Join(fmt.Sprintf("%s.users u ON u.id = s.user_id", r.schema)).
		Where(sq.Eq{"s.doc_id": docID, "s.can_read": true}).
		OrderBy("u.login ASC")

	sqlStr, args, _ := q.ToSql()
	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var login string
		if err := rows.Scan(&login); err != nil {
			return nil, err
		}
		out = append(out, login)
	}
	return out, rows.Err()
}
