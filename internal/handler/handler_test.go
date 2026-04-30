package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gopherledger/internal/domain"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeService struct {
	mock.Mock
}

func (s *fakeService) RegisterUser(login string, password string) (string, error) {
	args := s.Called(login, password)
	return args.String(0), args.Error(1)
}
func (s *fakeService) LoginUser(login string, password string) (string, error) {
	args := s.Called(login, password)
	return args.String(0), args.Error(1)
}
func (s *fakeService) CreateOrder(userID int64, number string) (*domain.Order, error) {
	args := s.Called(userID, number)
	return args.Get(0).(*domain.Order), args.Error(1)
}
func (s *fakeService) GetUserOrders(userID int64) ([]domain.Order, error) {
	args := s.Called(userID)
	return args.Get(0).([]domain.Order), args.Error(1)
}
func (s *fakeService) GetBalance(userID int64) (domain.Balance, error) {
	args := s.Called(userID)
	return args.Get(0).(domain.Balance), args.Error(1)
}
func (s *fakeService) Withdraw(userID int64, orderNumber string, sum float64) error {
	args := s.Called(userID, orderNumber, sum)
	return args.Error(0)
}
func (s *fakeService) GetWithdrawals(userID int64) ([]domain.Withdrawal, error) {
	args := s.Called(userID)
	return args.Get(0).([]domain.Withdrawal), args.Error(1)
}
func (s *fakeService) GetStats() (*domain.Stat, error) {
	args := s.Called()
	return args.Get(0).(*domain.Stat), args.Error(1)
}

// =========================================

func TestRegister(t *testing.T) {
	tests := []struct {
		name            string
		method, path    string
		login, password string
		setupMock       func(s *fakeService)
		expectedStatus  int
		expectedHeader  string
		expectedBody    httpError
	}{
		{name: "успех",
			method:   "POST",
			path:     "/api/user/register",
			login:    "test123",
			password: "qwerty",
			setupMock: func(s *fakeService) {
				s.On("RegisterUser", "test123", "qwerty").Return("tokEn", nil)
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "tokEn",
			expectedBody:   httpError{},
		},
		{name: "дублирование логина",
			method:   "POST",
			path:     "/api/user/register",
			login:    "test123",
			password: "qwerty",
			setupMock: func(s *fakeService) {
				s.On("RegisterUser", "test123", "qwerty").Return("", domain.ErrUserExists)
			},
			expectedStatus: http.StatusConflict,
			expectedHeader: "",
			expectedBody:   httpError{Code: "CONFLICT", UserMsg: "пользователь с таким именем уже существует"},
		},
		{name: "пустые поля",
			method:   "POST",
			path:     "/api/user/register",
			login:    "",
			password: "",
			setupMock: func(s *fakeService) {
				s.On("RegisterUser", "", "").Return("", domain.ErrInvalidData)
			},
			expectedStatus: http.StatusBadRequest,
			expectedHeader: "",
			expectedBody:   httpError{Code: "BAD_REQUEST", UserMsg: "неверный формат учетных данных"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			body, _ := json.Marshal(auth{
				Login:    tt.login,
				Password: tt.password,
			})
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.Register(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			bytesData, _ := io.ReadAll(ans.Body)
			var data httpError
			json.Unmarshal(bytesData, &data)

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)
			assert.Equal(t, tt.expectedBody, data)

			token := ans.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeader, token)
			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestLogin(t *testing.T) {
	tests := []struct {
		name            string
		method, path    string
		login, password string
		setupMock       func(s *fakeService)
		expectedStatus  int
		expectedHeader  string
		expectedBody    httpError
	}{
		{name: "успех",
			method:   "POST",
			path:     "/api/user/login",
			login:    "test123",
			password: "qwerty",
			setupMock: func(s *fakeService) {
				s.On("LoginUser", "test123", "qwerty").Return("tokEn", nil)
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "tokEn",
			expectedBody:   httpError{},
		},
		{name: "неверные данные",
			method:   "POST",
			path:     "/api/user/login",
			login:    "test123",
			password: "qwerty",
			setupMock: func(s *fakeService) {
				s.On("LoginUser", "test123", "qwerty").Return("", domain.ErrInvalidPassword)
			},
			expectedStatus: http.StatusUnauthorized,
			expectedHeader: "",
			expectedBody:   httpError{Code: "UNAUTHORIZED", UserMsg: "неверные учетные данные"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			body, _ := json.Marshal(auth{
				Login:    tt.login,
				Password: tt.password,
			})
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.Login(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			bytesData, _ := io.ReadAll(ans.Body)
			var data httpError
			json.Unmarshal(bytesData, &data)

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)
			assert.Equal(t, tt.expectedBody, data)

			token := ans.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeader, token)
			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name           string
		method, path   string
		userID         int64
		number         string
		setupMock      func(s *fakeService)
		expectedStatus int
		expectedHeader string
		expectedBody   httpError
	}{
		{name: "успех",
			method: "POST",
			path:   "/api/user/orders",
			userID: 123,
			number: "4532015112830366",
			setupMock: func(s *fakeService) {
				s.On("CreateOrder", int64(123), "4532015112830366").Return(&domain.Order{}, nil)
			},
			expectedStatus: http.StatusAccepted,
			expectedHeader: "",
			expectedBody:   httpError{},
		},
		{name: "неверный номер",
			method: "POST",
			path:   "/api/user/orders",
			userID: 123,
			number: "453201511283036",
			setupMock: func(s *fakeService) {
				s.On("CreateOrder", int64(123), "453201511283036").Return(&domain.Order{}, domain.ErrInvalidOrder)
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedHeader: "",
			expectedBody:   httpError{Code: "UNPROCESSABLE_ENTITY", UserMsg: "номер заказа не прошёл проверку Луна"},
		},
		{name: "конфликт",
			method: "POST",
			path:   "/api/user/orders",
			userID: 123,
			number: "4532015112830366",
			setupMock: func(s *fakeService) {
				s.On("CreateOrder", int64(123), "4532015112830366").Return(&domain.Order{}, domain.ErrOrderExists)
			},
			expectedStatus: http.StatusConflict,
			expectedHeader: "",
			expectedBody:   httpError{Code: "CONFLICT", UserMsg: "заказ принадлежит другому пользователю"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			body := bytes.NewReader([]byte(fmt.Sprintf("%s", tt.number)))
			request := httptest.NewRequest(tt.method, tt.path, body)
			ctx := request.Context()
			ctx = context.WithValue(ctx, CtxKeyUserID, tt.userID)
			request = request.WithContext(ctx)
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.CreateOrder(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)

			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestGetOrder(t *testing.T) {
	tests := []struct {
		name           string
		method, path   string
		userID         int64
		setupMock      func(s *fakeService)
		expectedStatus int
		expectedHeader string
		expectedBody   []domain.Order
	}{
		{name: "с заказами",
			method: "GET",
			path:   "/api/user/orders",
			userID: 123,
			setupMock: func(s *fakeService) {
				s.On("GetUserOrders", int64(123)).Return([]domain.Order{{}}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "",
			expectedBody:   []domain.Order{{}},
		},
		{name: "без заказов",
			method: "GET",
			path:   "/api/user/orders",
			userID: 123,
			setupMock: func(s *fakeService) {
				s.On("GetUserOrders", int64(123)).Return([]domain.Order(nil), nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedHeader: "",
			expectedBody:   []domain.Order(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte{}))
			ctx := request.Context()
			ctx = context.WithValue(ctx, CtxKeyUserID, tt.userID)
			request = request.WithContext(ctx)
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.GetOrders(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			bytesData, _ := io.ReadAll(ans.Body)
			var data []domain.Order
			json.Unmarshal(bytesData, &data)

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)
			assert.Equal(t, tt.expectedBody, data)

			token := ans.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeader, token)
			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestGetBalance(t *testing.T) {
	tests := []struct {
		name           string
		method, path   string
		userID         int64
		setupMock      func(s *fakeService)
		expectedStatus int
		expectedHeader string
		expectedBody   domain.Balance
	}{
		{name: "успех",
			method: "GET",
			path:   "/api/user/balance",
			userID: 123,
			setupMock: func(s *fakeService) {
				s.On("GetBalance", int64(123)).Return(domain.Balance{Current: 67, Withdrawn: 52}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "",
			expectedBody: domain.Balance{
				Current:   67,
				Withdrawn: 52,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte{}))
			ctx := request.Context()
			ctx = context.WithValue(ctx, CtxKeyUserID, tt.userID)
			request = request.WithContext(ctx)
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.GetBalance(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			bytesData, _ := io.ReadAll(ans.Body)
			var data domain.Balance
			json.Unmarshal(bytesData, &data)

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)
			assert.Equal(t, tt.expectedBody, data)

			token := ans.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeader, token)
			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestWithdraw(t *testing.T) {
	tests := []struct {
		name           string
		method, path   string
		userID         int64
		orderNumber    string
		sum            float64
		setupMock      func(s *fakeService)
		expectedStatus int
		expectedHeader string
		expectedBody   httpError
	}{
		{name: "успех",
			method:      "POST",
			path:        "/api/user/balance/withdraw",
			userID:      123,
			orderNumber: "4532015112830366",
			sum:         228,
			setupMock: func(s *fakeService) {
				s.On("Withdraw", int64(123), "4532015112830366", float64(228)).Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "",
			expectedBody:   httpError{},
		},
		{name: "нехватка баллов",
			method:      "POST",
			path:        "/api/user/balance/withdraw",
			userID:      123,
			orderNumber: "4532015112830366",
			sum:         228,
			setupMock: func(s *fakeService) {
				s.On("Withdraw", int64(123), "4532015112830366", float64(228)).Return(domain.ErrInsufficientFunds)
			},
			expectedStatus: http.StatusPaymentRequired,
			expectedHeader: "",
			expectedBody:   httpError{Code: "PAYMENT_REQUIRED", UserMsg: "недостаточно средств"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			withdrawal := domain.Withdrawal{
				ID:          67,
				UserID:      tt.userID,
				OrderNumber: tt.orderNumber,
				Sum:         tt.sum,
				ProcessedAt: time.Time{},
			}
			body, _ := json.Marshal(withdrawal)
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			ctx := request.Context()
			ctx = context.WithValue(ctx, CtxKeyUserID, tt.userID)
			request = request.WithContext(ctx)
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.Withdraw(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)

			token := ans.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeader, token)
			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestGetWithdrawals(t *testing.T) {
	tests := []struct {
		name           string
		method, path   string
		userID         int64
		setupMock      func(s *fakeService)
		expectedStatus int
		expectedHeader string
		expectedBody   []domain.Withdrawal
	}{
		{name: "с записями",
			method: "GET",
			path:   "/api/user/GetWithdrawals",
			userID: 123,
			setupMock: func(s *fakeService) {
				s.On("GetWithdrawals", int64(123)).Return([]domain.Withdrawal{{}}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "",
			expectedBody:   []domain.Withdrawal{{}},
		},
		{name: "без записей",
			method: "GET",
			path:   "/api/user/GetWithdrawals",
			userID: 123,
			setupMock: func(s *fakeService) {
				s.On("GetWithdrawals", int64(123)).Return([]domain.Withdrawal(nil), nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedHeader: "",
			expectedBody:   []domain.Withdrawal(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte{}))
			ctx := request.Context()
			ctx = context.WithValue(ctx, CtxKeyUserID, tt.userID)
			request = request.WithContext(ctx)
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.GetWithdrawals(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			bytesData, _ := io.ReadAll(ans.Body)
			var data []domain.Withdrawal
			json.Unmarshal(bytesData, &data)

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)
			assert.Equal(t, tt.expectedBody, data)

			token := ans.Header.Get("Authorization")
			assert.Equal(t, tt.expectedHeader, token)
			mockService.AssertExpectations(t)
		})

	}
}

// =========================================

func TestExportStats(t *testing.T) {
	tests := []struct {
		name           string
		method, path   string
		setupMock      func(s *fakeService)
		expectedStatus int
	}{
		{name: "успех",
			method: "POST",
			path:   "/api/stats/export",
			setupMock: func(s *fakeService) {
				s.On("GetStats").Return(&domain.Stat{
					UserCount:     42,
					OrdersCount:   52,
					TotalAccrual:  67,
					TotalWithdraw: 78,
					GeneratedAt:   time.Time{},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(fakeService)
			tt.setupMock(mockService)
			handler := New(mockService)
			request := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte{}))
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			handler.ExportStats(response, request)

			ans := response.Result()
			defer ans.Body.Close()

			assert.Equal(t, tt.expectedStatus, ans.StatusCode)
			defer os.Remove("stats.txt")
			_, err := os.Stat("stats.txt")
			assert.NoError(t, err)
			mockService.AssertExpectations(t)
		})

	}
}
