package store

import (
	"context"
	"errors"
	"fmt"
	"gopherledger/internal/auth"
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

func (p *PostgresRepo) SaveToken(userID int64, token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			INSERT INTO auth(user_id, token)
			VALUES($1, $2)
			ON CONFLICT (user_id) DO UPDATE
			SET token = EXCLUDED.token
	`

	_, err := p.pool.Exec(ctx, query, userID, token)
	if err != nil {
		return fmt.Errorf("ошибка сохранения токена: %w", err)
	}
	return nil
}

func (p *PostgresRepo) GetUserByToken(token string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			SELECT user_id FROM auth
			WHERE token = $1
	`
	var id int64
	err := p.pool.QueryRow(ctx, query, token).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, auth.ErrInvalidToken
		}
		return 0, fmt.Errorf("ошибка получения ID по токену: %w", err)
	}
	return id, nil
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
		Accrual:    0,
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

func (p *PostgresRepo) UpdateOrderStatus(number, status string, accrual float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			SELECT user_id, status FROM orders
			WHERE number = $1
			FOR UPDATE
	`
	var userID int64
	var currentStatus string

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка создания транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, query, number).Scan(&userID, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("заказ не найден: %w", err)
	}
	if err != nil {
		return fmt.Errorf("ошибка бд при получении заказа: %w", err)
	}

	if currentStatus == domain.OrderStatusProcessed || currentStatus == domain.OrderStatusInvalid {
		return nil
	}

	queryUpdateStatus := `
					UPDATE orders
					SET status = $1
					WHERE number = $2
		`
	_, err = tx.Exec(ctx, queryUpdateStatus, status, number)
	if err != nil {
		return fmt.Errorf("ошибка обновления заказа: %w", err)
	}

	if status != domain.OrderStatusProcessed || accrual <= 0 {
		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("ошибка коммита транзакции: %w", err)
		}
		return nil
	}

	queryUpdateBalance := `
					UPDATE balances
					SET current = current + $1
					WHERE user_id = $2
		`

	tag, err := tx.Exec(ctx, queryUpdateBalance, decimal.NewFromFloat(accrual), userID)
	if err != nil {
		return fmt.Errorf("ошибка обновления баланса: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("ошибка обработки: не удалось обновить баланс")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}
	return nil
}

func (p *PostgresRepo) GetBalance(userID int64) (domain.Balance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			SELECT current, withdrawn FROM balances
			WHERE user_id = $1
	`
	var balance domain.Balance

	err := p.pool.QueryRow(ctx, query, userID).Scan(&balance.Current, &balance.Withdrawn)
	if err != nil {
		return domain.Balance{}, fmt.Errorf("ошибка поиска баланса: %w", err)
	}

	return balance, nil
}

func (p *PostgresRepo) Withdraw(userID int64, orderNumber string, sum float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			UPDATE balances
			SET current = current - $1, withdrawn = withdrawn + $1
			WHERE user_id = $2 AND current >= $1
	`
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка создания транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	decSum := decimal.NewFromFloat(sum)
	tag, err := tx.Exec(ctx, query, decSum, userID)
	if err != nil {
		return fmt.Errorf("ошибка выполнения транзакции: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ошибка при списании: %w", domain.ErrInsufficientFunds)
	}

	queryWithdrawal := `
					INSERT INTO withdrawals(user_id, order_number, sum)
					VALUES ($1, $2, $3)
	`
	tag, err = tx.Exec(ctx, queryWithdrawal, userID, orderNumber, decSum)
	if err != nil {
		return fmt.Errorf("ошибка выполнения транзакции: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("ошибка выполнения транзакции: не удалось записать операцию")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}
	return nil
}

func (p *PostgresRepo) GetWithdrawals(userID int64) ([]domain.Withdrawal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	query := `
			SELECT id, user_id, order_number, sum, processed_at FROM withdrawals
			WHERE user_id = $1
			ORDER BY processed_at DESC
	`
	withdrawals := make([]domain.Withdrawal, 0)

	rows, err := p.pool.Query(ctx, query, userID)
	if err != nil {
		return withdrawals, fmt.Errorf("ошибка поиска списаний: %w", err)
	}
	defer rows.Close()

	var w domain.Withdrawal
	for rows.Next() {
		err = rows.Scan(&w.ID, &w.UserID, &w.OrderNumber, &w.Sum, &w.ProcessedAt)
		if err != nil {
			return make([]domain.Withdrawal, 0), fmt.Errorf("ошибка сканирования списаний: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	if err := rows.Err(); err != nil {
		return make([]domain.Withdrawal, 0), fmt.Errorf("ошибка при получении списаний: %w", err)
	}
	return withdrawals, nil
}

func (p *PostgresRepo) GetStats() (*domain.Stat, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.ctxTimeout)
	defer cancel()

	stat := &domain.Stat{}
	txOptions := pgx.TxOptions{
		IsoLevel:       pgx.RepeatableRead,
		AccessMode:     pgx.ReadOnly,
		DeferrableMode: pgx.Deferrable,
	}
	tx, err := p.pool.BeginTx(ctx, txOptions)
	if err != nil {
		return stat, fmt.Errorf("ошибка создания транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	queryGetOrders := `SELECT 
					COUNT(*) FILTER (WHERE status = $1),
					COUNT(*) FILTER (WHERE status = $2),
					COUNT(*) FILTER (WHERE status = $3),
					COUNT(*) FILTER (WHERE status = $4),
					COUNT(*),
					COALESCE(sum(accrual), 0)
					FROM orders
	`
	err = tx.QueryRow(
		ctx,
		queryGetOrders,
		domain.OrderStatusNew,
		domain.OrderStatusProcessing,
		domain.OrderStatusProcessed,
		domain.OrderStatusInvalid,
	).Scan(
		&stat.OrdersDistribution.NEW,
		&stat.OrdersDistribution.PROCESSING,
		&stat.OrdersDistribution.PROCESSED,
		&stat.OrdersDistribution.INVALID,
		&stat.OrdersCount,
		&stat.TotalAccrual,
	)
	if err != nil {
		return &domain.Stat{}, fmt.Errorf("ошибка выполнения транзакции: %w", err)
	}

	queryGetUsers := `SELECT 
					(SELECT COUNT(*) FROM users),
					COALESCE(SUM(sum), 0)
					FROM withdrawals
	`
	err = tx.QueryRow(
		ctx,
		queryGetUsers,
	).Scan(
		&stat.UserCount,
		&stat.TotalWithdraw,
	)
	if err != nil {
		return &domain.Stat{}, fmt.Errorf("ошибка выполнения транзакции: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &domain.Stat{}, fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	stat.GeneratedAt = time.Now()
	return stat, nil
}
