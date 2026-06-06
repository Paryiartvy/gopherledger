package store

import (
	"context"
	"errors"
	"fmt"
	"gopherledger/internal/domain"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
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

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return &domain.User{}, fmt.Errorf("ошибка создания транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, query, login, passwordHash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return &domain.User{}, fmt.Errorf("registration error: %w", domain.ErrUserExists)
		}
		return &domain.User{}, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}

	queryBalance := "INSERT INTO balances(user_id) VALUES($1)"

	_, err = tx.Exec(ctx, queryBalance, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return &domain.User{}, fmt.Errorf("registration error: %w", domain.ErrUserExists)
		}
		return &domain.User{}, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &domain.User{}, fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	user := domain.User{
		ID:           id,
		Login:        login,
		PasswordHash: passwordHash,
	}

	return &user, nil
}

func (p *PostgresRepo) GetUserByLogin(login string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := "SELECT id, login, password_hash FROM users WHERE login = $1"
	user := &domain.User{}

	err := p.pool.QueryRow(ctx, query, login).Scan(&user.ID, &user.Login, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.User{}, fmt.Errorf("user find error: %w", domain.ErrUserNotFound)
		}
		return &domain.User{}, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}

	return user, nil
}

func (p *PostgresRepo) CreateOrder(userID int64, number string) (*domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			INSERT INTO orders(user_id, number, status) VALUES($1, $2, $3) 
			ON CONFLICT (number) DO NOTHING RETURNING id
	`
	var id int64

	err := p.pool.QueryRow(ctx, query, userID, number, domain.OrderStatusNew).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		var ownerID int64
		queryUnique := "SELECT user_id FROM orders WHERE number = $1"
		errSelect := p.pool.QueryRow(ctx, queryUnique, number).Scan(&ownerID)
		if errSelect != nil {
			return &domain.Order{}, fmt.Errorf("ошибка проверки владельца заказа: %w", errSelect)
		}

		if ownerID == userID {
			return &domain.Order{}, fmt.Errorf("order create error: %w", domain.ErrOrderOwnedByUser)
		}

		return &domain.Order{}, fmt.Errorf("order create error: %w", domain.ErrOrderExists)
	}
	if err != nil {
		return &domain.Order{}, fmt.Errorf("ошибка бд при создании заказа: %w", err)
	}

	order := domain.Order{
		ID:         id,
		UserID:     userID,
		Number:     number,
		Status:     domain.OrderStatusNew,
		Accrual:    decimal.Zero,
		UploadedAt: time.Now(),
	}
	return &order, nil
}

func (p *PostgresRepo) GetUserOrders(userID int64) ([]domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			SELECT id, user_id, number, status, accrual, uploaded_at FROM orders
			WHERE user_id = $1
			ORDER BY uploaded_at DESC
	`
	orders := make([]domain.Order, 0)

	rows, err := p.pool.Query(ctx, query, userID)
	if err != nil {
		return orders, fmt.Errorf("ошибка поиска заказов: %w", err)
	}
	defer rows.Close()

	var o domain.Order
	for rows.Next() {
		err = rows.Scan(&o.ID, &o.UserID, &o.Number, &o.Status, &o.Accrual, &o.UploadedAt)
		if err != nil {
			return make([]domain.Order, 0), fmt.Errorf("ошибка сканирования заказов: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return make([]domain.Order, 0), fmt.Errorf("ошибка при получении заказа: %w", err)
	}
	return orders, nil
}

func (p *PostgresRepo) GetOrdersForProcessing() ([]domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			SELECT id, user_id, number, status, accrual, uploaded_at FROM orders
			WHERE status IN ($1, $2)
			ORDER BY uploaded_at DESC
	`
	orders := make([]domain.Order, 0)

	rows, err := p.pool.Query(ctx, query, domain.OrderStatusNew, domain.OrderStatusProcessing)
	if err != nil {
		return orders, fmt.Errorf("ошибка поиска заказов: %w", err)
	}
	defer rows.Close()

	var o domain.Order
	for rows.Next() {
		err = rows.Scan(&o.ID, &o.UserID, &o.Number, &o.Status, &o.Accrual, &o.UploadedAt)
		if err != nil {
			return make([]domain.Order, 0), fmt.Errorf("ошибка сканирования заказов: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return make([]domain.Order, 0), fmt.Errorf("ошибка при получении заказа: %w", err)
	}
	return orders, nil
}
