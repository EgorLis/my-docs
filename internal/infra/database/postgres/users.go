package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/EgorLis/my-docs/internal/domain"
	sq "github.com/Masterminds/squirrel"
)

func (r *PGRepo) CreateUser(ctx context.Context, login string, passHash []byte) (domain.User, error) {
	q := r.qb().Insert(fmt.Sprintf("%s.users", r.schema)).
		Columns("login", "pass_hash").
		Values(login, passHash).
		Suffix("RETURNING id, login, pass_hash, created_at")

	sqlStr, args, _ := q.ToSql()
	r.logSQL("CreateUser", sqlStr, args)

	start := time.Now()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Login, &u.PassHash, &u.CreatedAt); err != nil {
		r.logger.Printf("CreateUser scan error after %s: %v", time.Since(start), err)
		return domain.User{}, err
	}
	r.logger.Printf("CreateUser ok in %s id=%s login=%s", time.Since(start), u.ID, u.Login)
	return u, nil
}

func (r *PGRepo) UserByLogin(ctx context.Context, login string) (domain.User, error) {
	q := r.qb().Select("id", "login", "pass_hash", "created_at").
		From(fmt.Sprintf("%s.users", r.schema)).
		Where(sq.Eq{"login": login})

	sqlStr, args, _ := q.ToSql()
	r.logSQL("UserByLogin", sqlStr, args)

	start := time.Now()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Login, &u.PassHash, &u.CreatedAt); err != nil {
		r.logger.Printf("UserByLogin scan error after %s: %v", time.Since(start), err)
		return domain.User{}, err
	}
	r.logger.Printf("UserByLogin ok in %s id=%s", time.Since(start), u.ID)
	return u, nil
}

func (r *PGRepo) UserByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	q := r.qb().Select("id", "login", "pass_hash", "created_at").
		From(fmt.Sprintf("%s.users", r.schema)).
		Where(sq.Eq{"id": id})

	sqlStr, args, _ := q.ToSql()
	r.logSQL("UserByID", sqlStr, args)

	start := time.Now()
	row := r.pool.QueryRow(ctx, sqlStr, args...)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Login, &u.PassHash, &u.CreatedAt); err != nil {
		r.logger.Printf("UserByID scan error after %s: %v", time.Since(start), err)
		return domain.User{}, err
	}
	r.logger.Printf("UserByID ok in %s id=%s", time.Since(start), u.ID)
	return u, nil
}
