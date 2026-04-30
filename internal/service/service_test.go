package service

import (
	"gopherledger/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeStore struct {
	mock.Mock
}

func (s *fakeStore) CreateUser(login, passwordHash string) (*domain.User, error) {
	args := s.Called(login, passwordHash)
	return args.Get(0).(*domain.User), args.Error(1)
}
func (s *fakeStore) GetUserByLogin(login string) (*domain.User, error) {
	args := s.Called(login)
	return args.Get(0).(*domain.User), args.Error(1)
}
func (s *fakeStore) CreateOrder(userID int64, number string) (*domain.Order, error) {
	args := s.Called(userID, number)
	return args.Get(0).(*domain.Order), args.Error(1)
}
func (s *fakeStore) GetUserOrders(userID int64) ([]domain.Order, error) {
	args := s.Called(userID)
	return args.Get(0).([]domain.Order), args.Error(1)
}
func (s *fakeStore) GetOrdersForProcessing() ([]domain.Order, error) {
	args := s.Called()
	return args.Get(0).([]domain.Order), args.Error(1)
}
func (s *fakeStore) UpdateOrderStatus(number, status string, accrual float64) error {
	args := s.Called(number, status, accrual)
	return args.Error(0)
}
func (s *fakeStore) GetBalance(userID int64) (domain.Balance, error) {
	args := s.Called(userID)
	return args.Get(0).(domain.Balance), args.Error(1)
}
func (s *fakeStore) Withdraw(userID int64, orderNumber string, sum float64) error {
	args := s.Called(userID, orderNumber, sum)
	return args.Error(0)
}
func (s *fakeStore) GetWithdrawals(userID int64) ([]domain.Withdrawal, error) {
	args := s.Called(userID)
	return args.Get(0).([]domain.Withdrawal), args.Error(1)
}
func (s *fakeStore) GetStats() (*domain.Stat, error) {
	args := s.Called()
	return args.Get(0).(*domain.Stat), args.Error(1)
}

// =========================================
func TestRegisterUser(t *testing.T) {
	tests := []struct {
		name            string
		login, password string
		mockSetup       func(store *fakeStore)
		wantErr         bool
		errType         error
	}{
		{name: "успех",
			login:    "test123",
			password: "qwerty",
			mockSetup: func(s *fakeStore) {
				s.On("CreateUser", "test123", mock.AnythingOfType("string")).Return(&domain.User{
					ID:           67,
					Login:        "test123",
					PasswordHash: "reallyStrongPassword",
				}, nil)
			},
			wantErr: false,
			errType: nil,
		},
		{name: "повторная регистрация",
			login:    "test123",
			password: "qwerty",
			mockSetup: func(s *fakeStore) {
				s.On("CreateUser", "test123", mock.AnythingOfType("string")).Return(&domain.User{}, domain.ErrUserExists)
			},
			wantErr: true,
			errType: domain.ErrUserExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(fakeStore)
			tt.mockSetup(mockStore)
			service := New(mockStore)

			token, err := service.RegisterUser(tt.login, tt.password)

			if tt.wantErr {
				assert.ErrorIs(t, err, tt.errType)
			} else {
				assert.NoError(t, err)
				if uuid.Validate(token) != nil {
					t.Errorf("некорректный токен: %s", token)
				}
			}
			mockStore.AssertExpectations(t)
		})
	}
}

// =========================================
func TestLoginUser(t *testing.T) {
	tests := []struct {
		name            string
		login, password string
		mockSetup       func(store *fakeStore)
		wantErr         bool
		errType         error
	}{
		{name: "успех",
			login:    "test123",
			password: "qwerty",
			mockSetup: func(s *fakeStore) {
				s.On("GetUserByLogin", "test123").Return(&domain.User{
					ID:           67,
					Login:        "test123",
					PasswordHash: hash("qwerty"),
				}, nil)
			},
			wantErr: false,
			errType: nil,
		},
		{name: "неверный пароль",
			login:    "test123",
			password: "qwerty",
			mockSetup: func(s *fakeStore) {
				s.On("GetUserByLogin", "test123").Return(&domain.User{
					ID:           67,
					Login:        "test123",
					PasswordHash: "invalidHash",
				}, nil)
			},
			wantErr: true,
			errType: domain.ErrInvalidPassword,
		},
		{name: "пользователь не найден",
			login:    "test123",
			password: "qwerty",
			mockSetup: func(s *fakeStore) {
				s.On("GetUserByLogin", "test123").Return(&domain.User{}, domain.ErrUserNotFound)
			},
			wantErr: true,
			errType: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(fakeStore)
			tt.mockSetup(mockStore)
			service := New(mockStore)

			token, err := service.LoginUser(tt.login, tt.password)

			if tt.wantErr {
				assert.ErrorIs(t, err, tt.errType)
			} else {
				assert.NoError(t, err)
				if uuid.Validate(token) != nil {
					t.Errorf("некорректный токен: %s", token)
				}
			}
			mockStore.AssertExpectations(t)
		})
	}
}

// =========================================

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		number     string
		mockSetup  func(store *fakeStore)
		wantErr    bool
		errType    error
		wantAssert bool
	}{
		{name: "успех",
			userID: 52,
			number: "4532015112830366",
			mockSetup: func(s *fakeStore) {
				s.On("CreateOrder", int64(52), "4532015112830366").Return(&domain.Order{
					ID:         67,
					UserID:     52,
					Number:     "4532015112830366",
					Status:     domain.OrderStatusInvalid,
					Accrual:    42,
					UploadedAt: time.Time{},
				}, nil)
			},
			wantErr:    false,
			errType:    nil,
			wantAssert: true,
		},
		{name: "плохой заказ",
			userID: 52,
			number: "453201511283036",
			mockSetup: func(s *fakeStore) {
				s.On("CreateOrder", int64(52), "453201511283036").Return(&domain.Order{}, domain.ErrInvalidOrder)
			},
			wantErr:    true,
			errType:    domain.ErrInvalidOrder,
			wantAssert: false,
		},
		{name: "пользователь уже загрузил этот заказ",
			userID: 52,
			number: "4532015112830366",
			mockSetup: func(s *fakeStore) {
				s.On("CreateOrder", int64(52), "4532015112830366").Return(&domain.Order{}, domain.ErrOrderOwnedByUser)
			},
			wantErr:    true,
			errType:    domain.ErrOrderOwnedByUser,
			wantAssert: true,
		},
		{name: "это чужой заказ",
			userID: 52,
			number: "4532015112830366",
			mockSetup: func(s *fakeStore) {
				s.On("CreateOrder", int64(52), "4532015112830366").Return(&domain.Order{}, domain.ErrOrderExists)
			},
			wantErr:    true,
			errType:    domain.ErrOrderExists,
			wantAssert: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(fakeStore)
			tt.mockSetup(mockStore)
			service := New(mockStore)

			_, err := service.CreateOrder(tt.userID, tt.number)

			if tt.wantErr {
				assert.ErrorIs(t, err, tt.errType)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantAssert {
				mockStore.AssertExpectations(t)
			}
		})
	}
}

// =========================================

func TestWithdraw(t *testing.T) {
	tests := []struct {
		name        string
		userID      int64
		orderNumber string
		sum         float64
		mockSetup   func(store *fakeStore)
		wantErr     bool
		errType     error
		wantAssert  bool
	}{
		{name: "успех",
			userID:      52,
			orderNumber: "4532015112830366",
			sum:         100.,
			mockSetup: func(s *fakeStore) {
				s.On("Withdraw", int64(52), "4532015112830366", float64(100)).Return(nil)
			},
			wantErr:    false,
			errType:    nil,
			wantAssert: true,
		},
		{name: "нехватка баллов",
			userID:      52,
			orderNumber: "4532015112830366",
			sum:         100.,
			mockSetup: func(s *fakeStore) {
				s.On("Withdraw", int64(52), "4532015112830366", float64(100)).Return(domain.ErrInsufficientFunds)
			},
			wantErr:    true,
			errType:    domain.ErrInsufficientFunds,
			wantAssert: true,
		},
		{name: "плохой номер заказа",
			userID:      52,
			orderNumber: "453201511283036",
			sum:         100.,
			mockSetup: func(s *fakeStore) {
				s.On("Withdraw", int64(52), "453201511283036", float64(100)).Return(nil)
			},
			wantErr:    true,
			errType:    domain.ErrInvalidOrder,
			wantAssert: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(fakeStore)
			tt.mockSetup(mockStore)
			service := New(mockStore)

			err := service.Withdraw(tt.userID, tt.orderNumber, tt.sum)
			if tt.wantErr {
				assert.ErrorIs(t, err, tt.errType)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantAssert {
				mockStore.AssertExpectations(t)
			}
		})
	}
}

//GetUserOrders(userID int64) ([]domain.Order, error)
//GetBalance(userID int64) (domain.Balance, error)
//GetWithdrawals(userID int64) ([]domain.Withdrawal, error)
//GetStats() (*domain.Stat, error)
//StartAccrualWorker(ctx context.Context, accrualIntervalSeconds int, workers int)
//processAllPendingOrders(ctx context.Context, workers int)
//processOrder(ctx context.Context, number string)
