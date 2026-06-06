package store

import (
	"context"
	"errors"
	"fmt"
	"gopherledger/internal/domain"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepo struct {
	pool       *pgxpool.Pool
	ctxTimeout time.Duration
}

func New(pool *pgxpool.Pool, timeout time.Duration) *PostgresRepo {
	return &PostgresRepo{pool: pool, ctxTimeout: timeout}
}

func (p *PostgresRepo) CreateUser(login, passwordHash string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := "INSERT INTO users(login, password_hash) VALUES($1, $2) RETURNING id"
	var id int64

	err := p.pool.QueryRow(ctx, query, login, passwordHash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return &domain.User{}, fmt.Errorf("registration error: %w", domain.ErrUserExists)
		}
		return nil, err
	}

	user := domain.User{
		ID:           id,
		Login:        login,
		PasswordHash: passwordHash,
	}

	return &user, nil
}
