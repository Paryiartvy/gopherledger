// // Пакет store реализует хранилище данных в памяти.
// // Используйте отдельные мьютексы для независимых групп данных.
// // Реализуйте этот пакет самостоятельно.
package store

//
//import (
//	"fmt"
//	"gopherledger/internal/domain"
//	"slices"
//	"sync"
//	"time"
//)
//
//// Store хранит все данные приложения в памяти.
//// Добавьте средства защиты конкурентного доступа самостоятельно.
//type Store struct {
//	// ordersMu -> balanceMu -> userMu
//
//	userMu sync.RWMutex
//	// users хранит пользователей по их ID
//	users map[int64]*domain.User
//
//	// usersByLogin хранит пользователей по логину - для быстрого поиска при авторизации
//	usersByLogin map[string]*domain.User
//
//	// nextID используется для генерации уникальных числовых ID
//	nextID int64
//
//	ordersMu sync.RWMutex
//	// orders хранит заказы по номеру заказа
//	orders map[string]*domain.Order
//
//	balanceMu sync.RWMutex
//	// balances хранит текущий баланс каждого пользователя по его ID
//	balances map[int64]*domain.Balance
//
//	// withdrawals хранит историю списаний для каждого пользователя по его ID
//	withdrawals map[int64][]*domain.Withdrawal
//}
//
//New создаёт и возвращает новое пустое хранилище.
//func New() *Store {
//	return &Store{
//		users:        make(map[int64]*domain.User),
//		usersByLogin: make(map[string]*domain.User),
//		nextID:       1,
//		orders:       make(map[string]*domain.Order),
//		balances:     make(map[int64]*domain.Balance),
//		withdrawals:  make(map[int64][]*domain.Withdrawal),
//	}
//}
//
//// CreateUser добавляет нового пользователя.
//// Возвращает domain.ErrUserExists если логин уже занят.
//func (s *Store) CreateUser(login, passwordHash string) (*domain.User, error) {
//	s.userMu.Lock()
//	defer s.userMu.Unlock()
//	if _, ok := s.usersByLogin[login]; ok {
//		return &domain.User{}, fmt.Errorf("registration error: %w", domain.ErrUserExists)
//	}
//	user := domain.User{
//		ID:           s.nextID,
//		Login:        login,
//		PasswordHash: passwordHash,
//	}
//
//	s.usersByLogin[login] = &user
//	s.users[s.nextID] = &user
//	s.nextID += 1
//
//	return &user, nil
//}
//
//// GetUserByLogin возвращает пользователя по логину.
//// Возвращает domain.ErrUserNotFound если пользователь не найден.
//func (s *Store) GetUserByLogin(login string) (*domain.User, error) {
//	s.userMu.RLock()
//	defer s.userMu.RUnlock()
//
//	if user, ok := s.usersByLogin[login]; ok {
//		return user, nil
//	}
//	return &domain.User{}, fmt.Errorf("user find error: %w", domain.ErrUserNotFound)
//}
//
//// CreateOrder добавляет новый заказ для пользователя.
//// Возвращает domain.ErrOrderOwnedByUser если этот пользователь уже загружал этот номер.
//// Возвращает domain.ErrOrderExists если номер принадлежит другому пользователю.
//func (s *Store) CreateOrder(userID int64, number string) (*domain.Order, error) {
//	s.userMu.Lock()
//	nextID := s.nextID
//	s.nextID += 1
//	s.userMu.Unlock()
//
//	s.ordersMu.Lock()
//	defer s.ordersMu.Unlock()
//
//	if order, ok := s.orders[number]; ok {
//		if order.UserID == userID {
//			return &domain.Order{}, fmt.Errorf("order create error: %w", domain.ErrOrderOwnedByUser)
//		}
//
//		return &domain.Order{}, fmt.Errorf("order create error: %w", domain.ErrOrderExists)
//	}
//
//	order := domain.Order{
//		ID:         nextID,
//		UserID:     userID,
//		Number:     number,
//		Status:     domain.OrderStatusNew,
//		Accrual:    0,
//		UploadedAt: time.Now(),
//	}
//	s.orders[number] = &order
//	return &order, nil
//}
//
//// GetUserOrders возвращает все заказы пользователя, сначала новые.
//func (s *Store) GetUserOrders(userID int64) ([]domain.Order, error) {
//	orders := make([]domain.Order, 0)
//
//	s.ordersMu.RLock()
//	defer s.ordersMu.RUnlock()
//
//	for _, order := range s.orders {
//		if order.UserID == userID {
//			orders = append(orders, *order)
//		}
//	}
//	slices.SortFunc(orders, func(a, b domain.Order) int {
//		return b.UploadedAt.Compare(a.UploadedAt)
//	})
//	return orders, nil
//}
//
//// GetOrdersForProcessing возвращает все заказы в статусе NEW или PROCESSING.
//func (s *Store) GetOrdersForProcessing() ([]domain.Order, error) {
//	orders := make([]domain.Order, 0)
//
//	s.ordersMu.RLock()
//	defer s.ordersMu.RUnlock()
//
//	for _, order := range s.orders {
//		if order.Status == domain.OrderStatusNew || order.Status == domain.OrderStatusProcessing {
//			orders = append(orders, *order)
//		}
//	}
//	slices.SortFunc(orders, func(a, b domain.Order) int {
//		return b.UploadedAt.Compare(a.UploadedAt)
//	})
//	return orders, nil
//}
//
//// UpdateOrderStatus обновляет статус и начисление заказа.
//// Если статус PROCESSED и accrual > 0, баланс пользователя пополняется.
//func (s *Store) UpdateOrderStatus(number, status string, accrual float64) error {
//	s.ordersMu.Lock()
//	order, ok := s.orders[number]
//
//	if !ok {
//		s.ordersMu.Unlock()
//		return fmt.Errorf("ошибка при обновлении заказа: %w", domain.ErrInvalidOrder)
//	}
//	userID := order.UserID
//	order.Status = status
//	order.Accrual = accrual
//	s.ordersMu.Unlock()
//
//	if status == domain.OrderStatusProcessed && accrual > 0 {
//		s.balanceMu.Lock()
//		defer s.balanceMu.Unlock()
//
//		if balance, ok := s.balances[userID]; ok {
//			balance.Current += accrual
//		} else {
//			s.balances[userID] = &domain.Balance{Current: accrual, Withdrawn: 0.}
//		}
//	}
//	return nil
//}
//
//// GetBalance возвращает баланс пользователя.
//func (s *Store) GetBalance(userID int64) (domain.Balance, error) {
//	s.balanceMu.RLock()
//	defer s.balanceMu.RUnlock()
//
//	balance, ok := s.balances[userID]
//
//	if !ok {
//		return domain.Balance{Current: 0, Withdrawn: 0}, nil
//	}
//	return *balance, nil
//}
//
//// Withdraw списывает сумму с баланса и записывает операцию.
//// Возвращает domain.ErrInsufficientFunds если баланса не хватает.
//// Обе операции должны быть атомарны: либо обе успешны, либо ни одна.
//func (s *Store) Withdraw(userID int64, orderNumber string, sum float64) error {
//	s.userMu.Lock()
//	nextID := s.nextID
//	s.nextID += 1
//	s.userMu.Unlock()
//
//	s.balanceMu.Lock()
//	defer s.balanceMu.Unlock()
//
//	balance, ok := s.balances[userID]
//
//	if !ok {
//		balance = &domain.Balance{Current: 0, Withdrawn: 0}
//	}
//
//	if balance.Current < sum {
//		return fmt.Errorf("ошибка при списании: %w", domain.ErrInsufficientFunds)
//	}
//
//	balance.Current -= sum
//	balance.Withdrawn += sum
//	s.balances[userID] = balance
//	withdrawal := domain.Withdrawal{
//		ID:          nextID,
//		UserID:      userID,
//		OrderNumber: orderNumber,
//		Sum:         sum,
//		ProcessedAt: time.Now(),
//	}
//	s.withdrawals[userID] = append(s.withdrawals[userID], &withdrawal)
//
//	return nil
//}
//
//// GetWithdrawals возвращает историю списаний пользователя, сначала новые.
//func (s *Store) GetWithdrawals(userID int64) ([]domain.Withdrawal, error) {
//	withdrawals := make([]domain.Withdrawal, 0)
//
//	s.balanceMu.RLock()
//	defer s.balanceMu.RUnlock()
//
//	for _, withdrawal := range s.withdrawals[userID] {
//		withdrawals = append(withdrawals, *withdrawal)
//	}
//
//	slices.SortFunc(withdrawals, func(a, b domain.Withdrawal) int {
//		return b.ProcessedAt.Compare(a.ProcessedAt)
//	})
//
//	return withdrawals, nil
//}
//
//// GetStats возвращает статистику по хранилищу
//func (s *Store) GetStats() (*domain.Stat, error) {
//	// ordersMu -> balanceMu -> userMu
//	stat := &domain.Stat{}
//
//	s.userMu.RLock()
//	stat.UserCount = len(s.users)
//	s.userMu.RUnlock()
//
//	s.ordersMu.RLock()
//	stat.OrdersCount = len(s.orders)
//	for _, v := range s.orders {
//		switch v.Status {
//		case domain.OrderStatusNew:
//			stat.OrdersDistribution.NEW += 1
//		case domain.OrderStatusProcessing:
//			stat.OrdersDistribution.PROCESSING += 1
//		case domain.OrderStatusProcessed:
//			stat.OrdersDistribution.PROCESSED += 1
//		case domain.OrderStatusInvalid:
//			stat.OrdersDistribution.INVALID += 1
//		default:
//			return nil, fmt.Errorf("неизвестный статус у заказа: %s", v.Status)
//		}
//	}
//	s.ordersMu.RUnlock()
//
//	s.balanceMu.RLock()
//	for _, v := range s.balances {
//		stat.TotalAccrual += v.Current + v.Withdrawn
//		stat.TotalWithdraw += v.Withdrawn
//	}
//	s.balanceMu.RUnlock()
//
//	stat.GeneratedAt = time.Now()
//
//	return stat, nil
//}
