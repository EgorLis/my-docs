package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/google/uuid"
)

func (r *PGRepo) CreateDoc(ctx context.Context, meta domain.Document, jsonBody domain.DocJSON) (domain.Document, error) {
	// вставляем метаданные
	q := r.qb().Insert(fmt.Sprintf("%s.documents", r.schema)).
		Columns("owner_id", "name", "mime_type", "file", "public", "size_bytes", "storage_key", "content_sha256").
		Values(meta.OwnerID, meta.Name, meta.MIME, meta.File, meta.Public, meta.SizeBytes, meta.StorageKey, meta.SHA256).
		Suffix("RETURNING id, owner_id, name, mime_type, file, public, size_bytes, storage_key, content_sha256, version, created_at, updated_at")

	sqlStr, args, _ := q.ToSql()
	r.logSQL("CreateDoc", sqlStr, args)

	start := time.Now()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var out domain.Document
	if err := row.Scan(
		&out.ID, &out.OwnerID, &out.Name, &out.MIME, &out.File, &out.Public,
		&out.SizeBytes, &out.StorageKey, &out.SHA256, &out.Version, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		r.logger.Printf("CreateDoc scan error after %s: %v", time.Since(start), err)
		return domain.Document{}, err
	}
	r.logger.Printf("CreateDoc meta ok in %s id=%s name=%q", time.Since(start), out.ID, out.Name)

	// json (опционально)
	if jsonBody != nil {
		payload, err := json.Marshal(jsonBody)
		if err != nil {
			r.logger.Printf("CreateDoc marshal json error: %v", err)
			return domain.Document{}, err
		}
		qj := r.qb().Insert(fmt.Sprintf("%s.doc_json", r.schema)).
			Columns("doc_id", "body").Values(out.ID, payload)
		sqlStr, args, _ = qj.ToSql()
		r.logSQL("CreateDoc.json", sqlStr, args)

		startJ := time.Now()
		if _, err := r.pool.Exec(ctx, sqlStr, args...); err != nil {
			r.logger.Printf("CreateDoc.json exec error after %s: %v", time.Since(startJ), err)
			return domain.Document{}, err
		}
		r.logger.Printf("CreateDoc json ok in %s id=%s", time.Since(startJ), out.ID)
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
	r.logSQL("DocByID.meta", sqlStr, args)

	start := time.Now()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var d domain.Document
	if err := row.Scan(
		&d.ID, &d.OwnerID, &d.Name, &d.MIME, &d.File, &d.Public,
		&d.SizeBytes, &d.StorageKey, &d.SHA256,
		&d.Version, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		r.logger.Printf("DocByID meta scan error after %s: %v", time.Since(start), err)
		return domain.Document{}, nil, err
	}
	r.logger.Printf("DocByID meta ok in %s id=%s", time.Since(start), d.ID)

	// doc_json (может отсутствовать)
	var jsonRaw []byte
	qj := r.qb().Select("body").
		From(fmt.Sprintf("%s.doc_json", r.schema)).
		Where(sq.Eq{"doc_id": id})
	sqlStr, args, _ = qj.ToSql()
	r.logSQL("DocByID.json", sqlStr, args)

	startJ := time.Now()
	err := r.pool.QueryRow(ctx, sqlStr, args...).Scan(&jsonRaw)
	if err != nil {
		// not fatal: если нет строки — оставим json пустым
		r.logger.Printf("DocByID json scan warn after %s: %v", time.Since(startJ), err)
	}
	var dj domain.DocJSON
	if len(jsonRaw) > 0 {
		if e := json.Unmarshal(jsonRaw, &dj); e != nil {
			r.logger.Printf("DocByID json unmarshal error: %v", e)
		}
	}
	return d, dj, nil
}

func (r *PGRepo) DocDelete(ctx context.Context, id domain.DocID, owner domain.UserID) error {
	q := r.qb().Delete(fmt.Sprintf("%s.documents", r.schema)).
		Where(sq.And{sq.Eq{"id": id}, sq.Eq{"owner_id": owner}})
	sqlStr, args, _ := q.ToSql()
	r.logSQL("DocDelete", sqlStr, args)

	start := time.Now()
	tag, err := r.pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Printf("DocDelete exec error after %s: %v", time.Since(start), err)
		return err
	}
	ra := tag.RowsAffected()
	if ra == 0 {
		r.logger.Printf("DocDelete no rows affected in %s (doc not found or not owner)", time.Since(start))
		return sqlNoRowsErr("document not found or not owner")
	}
	r.logger.Printf("DocDelete ok in %s rows=%d", time.Since(start), ra)
	return nil
}

func sqlNoRowsErr(msg string) error { return fmt.Errorf(msg) }

// Выдаёт документы пользователя (свои + публичные + расшаренные)
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
		// неизвестный ключ — игнорируем
	}

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
	r.logSQL("DocsList", sqlStr, args)

	start := time.Now()
	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Printf("DocsList query error after %s: %v", time.Since(start), err)
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
			r.logger.Printf("DocsList scan error: %v", err)
			return nil, err
		}
		res = append(res, d)
	}
	if err := rows.Err(); err != nil {
		r.logger.Printf("DocsList rows error: %v", err)
		return nil, err
	}
	r.logger.Printf("DocsList ok in %s count=%d", time.Since(start), len(res))
	return res, nil
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
	r.logSQL("Touch", sqlStr, args)

	start := time.Now()
	_, err := r.pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Printf("Touch exec error after %s: %v", time.Since(start), err)
		return err
	}
	r.logger.Printf("Touch ok in %s id=%s", time.Since(start), id)
	return nil
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
	r.logSQL("UpsertReadGrant", sqlStr, args)

	start := time.Now()
	_, err := r.pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Printf("UpsertReadGrant exec error after %s: %v", time.Since(start), err)
		return err
	}
	r.logger.Printf("UpsertReadGrant ok in %s doc_id=%s login=%s", time.Since(start), docID, login)
	return nil
}

func (r *PGRepo) RemoveGrant(ctx context.Context, docID domain.DocID, login string) error {
	q := r.qb().Delete(fmt.Sprintf("%s.doc_shares", r.schema)).
		Where(sq.And{
			sq.Eq{"doc_id": docID},
			sq.Expr("user_id = (SELECT id FROM "+r.schema+".users WHERE login = ?)", login),
		})
	sqlStr, args, _ := q.ToSql()
	r.logSQL("RemoveGrant", sqlStr, args)

	start := time.Now()
	_, err := r.pool.Exec(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Printf("RemoveGrant exec error after %s: %v", time.Since(start), err)
		return err
	}
	r.logger.Printf("RemoveGrant ok in %s doc_id=%s login=%s", time.Since(start), docID, login)
	return nil
}

func (r *PGRepo) ListGrantedLogins(ctx context.Context, docID domain.DocID) ([]string, error) {
	q := r.qb().Select("u.login").
		From(fmt.Sprintf("%s.doc_shares s", r.schema)).
		Join(fmt.Sprintf("%s.users u ON u.id = s.user_id", r.schema)).
		Where(sq.Eq{"s.doc_id": docID, "s.can_read": true}).
		OrderBy("u.login ASC")

	sqlStr, args, _ := q.ToSql()
	r.logSQL("ListGrantedLogins", sqlStr, args)

	start := time.Now()
	rows, err := r.pool.Query(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Printf("ListGrantedLogins query error after %s: %v", time.Since(start), err)
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var login string
		if err := rows.Scan(&login); err != nil {
			r.logger.Printf("ListGrantedLogins scan error: %v", err)
			return nil, err
		}
		out = append(out, login)
	}
	if err := rows.Err(); err != nil {
		r.logger.Printf("ListGrantedLogins rows error: %v", err)
		return nil, err
	}
	r.logger.Printf("ListGrantedLogins ok in %s count=%d", time.Since(start), len(out))
	return out, nil
}
