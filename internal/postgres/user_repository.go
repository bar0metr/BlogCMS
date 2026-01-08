package postgres

import (
	"context"
	"database/sql"
	"errors"

	"blogcms/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) ByUsername(ctx context.Context, username string) (domain.User, error) {
	const q = `SELECT id, username, password_hash, created_at FROM users WHERE username=$1;`
	var u domain.User
	err := r.db.QueryRowContext(ctx, q, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}
