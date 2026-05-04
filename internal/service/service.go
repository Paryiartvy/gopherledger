// Пакет service содержит бизнес-логику приложения.
//
// Взаимодействие с хранилищем осуществляется через интерфейс.
// Определите этот интерфейс здесь, по месту использования.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"gopherledger/internal/auth"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"gopherledger/internal/domain"

	"golang.org/x/sync/errgroup"
)

type Repository interface {
	CreateUser(login, passwordHash string) (*domain.User, error)
	GetUserByLogin(login string) (*domain.User, error)
	CreateOrder(userID int64, number string) (*domain.Order, error)
	GetUserOrders(userID int64) ([]domain.Order, error)
	GetOrdersForProcessing() ([]domain.Order, error)
	UpdateOrderStatus(number, status string, accrual float64) error
	GetBalance(userID int64) (domain.Balance, error)
	Withdraw(userID int64, orderNumber string, sum float64) error
	GetWithdrawals(userID int64) ([]domain.Withdrawal, error)
	GetStats() (*domain.Stat, error)
}

// Service реализует бизнес-логику приложения.
// Замените поле repo в структуре на свой интерфейс.
//
// processingOrders хранит номера заказов, которые сейчас обрабатываются воркером.
// Защитите конкурентный доступ к этому полю самостоятельно.
type Service struct {
	repo Repository // замените на свой интерфейс в структуре

	ordersMu         sync.RWMutex
	processingOrders map[string]bool
}

// New создаёт Service.
func New(repo Repository) *Service {
	return &Service{
		repo:             repo,
		processingOrders: make(map[string]bool),
	}
}

// ---------------------------------------------------------------------------
// Методы бизнес-логики - реализуйте самостоятельно
// ---------------------------------------------------------------------------
func hash(item string) string {
	h := sha256.New()
	h.Write([]byte(item))
	hashBytes := h.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

// RegisterUser регистрирует нового пользователя и возвращает токен аутентификации.
// Хешируйте пароль перед сохранением с помощью crypto/sha256.
func (s *Service) RegisterUser(login, password string) (string, error) {
	if strings.TrimSpace(login) == "" || strings.TrimSpace(password) == "" {
		return "", domain.ErrInvalidData
	}
	passwordHash := hash(password)

	user, err := s.repo.CreateUser(login, passwordHash)

	if err != nil {
		return "", err
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("ошибка генерации токена при регистрации: %w", err)
	}
	return token, nil
}

// LoginUser проверяет учётные данные и возвращает токен аутентификации.
func (s *Service) LoginUser(login, password string) (string, error) {
	user, err := s.repo.GetUserByLogin(login)
	if err != nil {
		return "", err
	}
	passwordHash := hash(password)
	if passwordHash == user.PasswordHash {
		token, err := auth.GenerateToken(user.ID)
		if err != nil {
			return "", fmt.Errorf("ошибка генерации токена при аутентификации: %w", err)
		}
		return token, nil
	}
	return "", domain.ErrInvalidPassword
}

// CreateOrder проверяет номер заказа по алгоритму Луна и сохраняет заказ.
func (s *Service) CreateOrder(userID int64, number string) (*domain.Order, error) {
	isCorrect := validateLuhn(number)
	if !isCorrect {
		return &domain.Order{}, domain.ErrInvalidOrder
	}

	order, err := s.repo.CreateOrder(userID, number)
	if err != nil {
		return &domain.Order{}, err
	}

	return order, nil
}

// GetUserOrders возвращает все заказы пользователя.
func (s *Service) GetUserOrders(userID int64) ([]domain.Order, error) {
	orders, err := s.repo.GetUserOrders(userID)
	if err != nil {
		return make([]domain.Order, 0), err
	}
	return orders, nil
}

// GetBalance возвращает текущий баланс пользователя.
func (s *Service) GetBalance(userID int64) (domain.Balance, error) {
	balance, err := s.repo.GetBalance(userID)
	if err != nil {
		return domain.Balance{}, err
	}

	return balance, nil
}

// Withdraw проверяет номер заказа по алгоритму Луна и списывает сумму с баланса.
func (s *Service) Withdraw(userID int64, orderNumber string, sum float64) error {
	isCorrect := validateLuhn(orderNumber)
	if !isCorrect {
		return domain.ErrInvalidOrder
	}

	err := s.repo.Withdraw(userID, orderNumber, sum)
	return err
}

// GetWithdrawals возвращает историю списаний пользователя.
func (s *Service) GetWithdrawals(userID int64) ([]domain.Withdrawal, error) {
	withdrawals, err := s.repo.GetWithdrawals(userID)
	if err != nil {
		return make([]domain.Withdrawal, 0), err
	}

	return withdrawals, nil
}

// GetStats возвращает статистику по всему хранилищу
func (s *Service) GetStats() (*domain.Stat, error) {
	stat, err := s.repo.GetStats()
	if err != nil {
		return stat, err
	}
	return stat, nil
}

// validateLuhn проверяет контрольную сумму номера заказа по алгоритму Луна.
// Вызывается при загрузке заказа и при списании баллов.
func validateLuhn(number string) bool {
	sum := 0

	for i, j := len(number)-1, 1; i >= 0; i, j = i-1, j+1 {
		if j%2 == 0 {
			num := int(number[i]-'0') * 2
			if num > 9 {
				num -= 9
			}
			sum += num
		} else {
			num := int(number[i] - '0')
			sum += num
		}
	}
	return sum%10 == 0
}

// ---------------------------------------------------------------------------
// Воркер начислений
//
// StartAccrualWorker предоставлен. Реализуйте processAllPendingOrders
// и processOrder самостоятельно.
//
// Это самая интересная часть проекта: конкурентная обработка заказов.
// Подумайте, как защитить доступ к processingOrders из нескольких горутин.
// ---------------------------------------------------------------------------

// StartAccrualWorker запускает фоновый цикл, который каждые 3 секунды
// передаёт необработанные заказы в processAllPendingOrders.
// Останавливается при отмене ctx.
func (s *Service) StartAccrualWorker(ctx context.Context, accrualIntervalSeconds int, workers int) {
	ticker := time.NewTicker(time.Duration(accrualIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processAllPendingOrders(ctx, workers)
		}
	}
}

// processAllPendingOrders получает заказы для обработки и запускает горутины.
// Реализуйте самостоятельно.
func (s *Service) processAllPendingOrders(ctx context.Context, workers int) {

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	orders, err := s.repo.GetOrdersForProcessing()

	if err != nil {
		log.Printf("ошибка получения заказов: %v", err)
		return
	}

	for _, order := range orders {
		s.ordersMu.Lock()
		if s.processingOrders[order.Number] {
			s.ordersMu.Unlock()
			continue
		}

		s.processingOrders[order.Number] = true
		s.ordersMu.Unlock()

		err = s.repo.UpdateOrderStatus(order.Number, domain.OrderStatusProcessing, 0)
		if err != nil {
			log.Printf("%v", err)
		}

		number := order.Number
		g.Go(func() error {
			s.processOrder(gctx, number)
			return nil
		})

	}

	if err := g.Wait(); err != nil {
		log.Printf("воркер: ошибка группы: %v", err)
	}
}

// processOrder обрабатывает один заказ. Реализуйте самостоятельно.
// Используйте вспомогательные функции ниже для генерации случайных значений.
func (s *Service) processOrder(ctx context.Context, number string) {
	defer func() {
		s.ordersMu.Lock()
		delete(s.processingOrders, number)
		s.ordersMu.Unlock()
	}()

	delay := randomDelay()

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return
	}

	invalid := isInvalid()
	if invalid {
		err := s.repo.UpdateOrderStatus(number, domain.OrderStatusInvalid, 0)
		if err != nil {
			log.Printf("%v", err)
		}

		return
	}

	err := s.repo.UpdateOrderStatus(number, domain.OrderStatusProcessed, randomAccrual())
	if err != nil {
		log.Printf("%v", err)
	}

	return
}

// ---------------------------------------------------------------------------
// Вспомогательные функции - предоставлены
// ---------------------------------------------------------------------------

// randomAccrual возвращает случайное начисление от 10 до 500 баллов.
func randomAccrual() float64 {
	return float64(rand.Intn(491) + 10)
}

// randomDelay возвращает случайную задержку от 2 до 6 секунд.
func randomDelay() time.Duration {
	return time.Duration(rand.Intn(5)+2) * time.Second
}

// isInvalid возвращает true примерно в 10% случаев.
func isInvalid() bool {
	return rand.Intn(10) == 0
}
