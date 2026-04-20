// Пакет handler содержит HTTP-обработчики.
//
// Взаимодействие с бизнес-логикой осуществляется через интерфейс.
// Определите этот интерфейс здесь, по месту использования.
// Реализуйте все обработчики самостоятельно.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"gopherledger/internal/domain"
	"io"
	"log"
	"net/http"
	"time"
)

type Service interface {
	RegisterUser(login string, password string) (string, error)
	LoginUser(login string, password string) (string, error)
	CreateOrder(userID int64, number string) (*domain.Order, error)
	GetUserOrders(userID int64) ([]domain.Order, error)
	GetBalance(userID int64) (domain.Balance, error)
	Withdraw(userID int64, orderNumber string, sum float64) error
	GetWithdrawals(userID int64) ([]domain.Withdrawal, error)
	//StartAccrualWorker(ctx context.Context)
}

// Handler хранит зависимость от бизнес-логики.
// Замените interface{} на свой интерфейс.
type Handler struct {
	svc Service
}

// New создаёт Handler.
func New(svc Service) *Handler {
	return &Handler{svc: svc}
}

// ---------------------------------------------------------------------------
// Вспомогательные функции для ответов - предоставлены
// ---------------------------------------------------------------------------
type httpError struct {
	Code    string `json:"code"`
	UserMsg string `json:"message"`
}

// writeError записывает JSON-ответ с ошибкой.
// Клиент видит только userMsg. Внутренние детали пишутся только в лог.
// Прочитайте ТЗ и создайте структуру тела ответа самостоятельно.
func writeError(w http.ResponseWriter, status int, code, userMsg string, internalErr error) {
	if internalErr != nil {
		log.Printf("ошибка code=%s status=%d: %v", code, status, internalErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	userError := httpError{
		Code:    code,
		UserMsg: userMsg,
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(userError); err != nil {
		log.Printf("ошибка сериализации: %s", err)
	}
}

// writeJSON записывает успешный JSON-ответ.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Обработчики - реализуйте самостоятельно
// ---------------------------------------------------------------------------
type auth struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Register обрабатывает POST /api/user/register.
// При успехе: 200 OK, заголовок Authorization с токеном.
// При дублировании логина: 409 Conflict.
// При некорректных данных: 400 Bad Request.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	defer func() {
		err := r.Body.Close()
		if err != nil {
			log.Printf("ошибка при закрытии тела запроса при регистрации: %s", err)
		}
	}()

	registerInfo := auth{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&registerInfo); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "неверный формат учетных данных", err)
		return
	}

	token, err := h.svc.RegisterUser(registerInfo.Login, registerInfo.Password)
	if err != nil {
		if errors.Is(err, domain.ErrUserExists) {
			writeError(w, http.StatusConflict, "CONFLICT", "пользователь с таким именем уже существует", err)
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", err)
		}
		return
	}

	w.Header().Set("Authorization", token)
	w.WriteHeader(http.StatusOK)
}

// Login обрабатывает POST /api/user/login.
// При успехе: 200 OK, заголовок Authorization с токеном.
// При неверных данных: 401 Unauthorized.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	defer func() {
		err := r.Body.Close()
		if err != nil {
			log.Printf("ошибка при закрытии тела запроса при логине: %s", err)
		}
	}()

	loginInfo := auth{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&loginInfo); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "неверный формат учетных данных", err)
		return
	}

	token, err := h.svc.LoginUser(loginInfo.Login, loginInfo.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPassword) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "неверные учетные данные", err)
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", err)
		}
		return
	}

	w.Header().Set("Authorization", token)
	w.WriteHeader(http.StatusOK)
}

// CreateOrder обрабатывает POST /api/user/orders.
// Тело запроса: номер заказа в виде обычного текста.
// 202 Accepted  - новый заказ принят в обработку.
// 200 OK        - заказ уже загружен этим пользователем.
// 409 Conflict  - заказ принадлежит другому пользователю.
// 422 Unprocessable Entity - номер не прошёл проверку Луна.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	defer func() {
		err := r.Body.Close()
		if err != nil {
			log.Printf("ошибка при закрытии тела запроса при создании заказа: %s", err)
		}
	}()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "ошибка при чтении тела запроса при создании заказа", err)
		return
	}
	userId, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
		return
	}

	number := string(data)
	_, err = h.svc.CreateOrder(userId, number)

	if err != nil {
		if errors.Is(err, domain.ErrInvalidOrder) {
			writeError(w, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY", "номер заказа не прошёл проверку Луна", err)
		} else if errors.Is(err, domain.ErrOrderExists) {
			writeError(w, http.StatusConflict, "CONFLICT", "заказ принадлежит другому пользователю", err)
		} else if errors.Is(err, domain.ErrOrderOwnedByUser) {
			w.WriteHeader(http.StatusOK)
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", err)
		}
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type userOrder struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// GetOrders обрабатывает GET /api/user/orders.
// 200 OK с JSON-массивом заказов или 204 No Content если заказов нет.
func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request) {
	defer func() {
		err := r.Body.Close()
		if err != nil {
			log.Printf("ошибка при закрытии тела запроса при получении заказов: %s", err)
		}
	}()

	userId, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
		return
	}

	ordersInternal, err := h.svc.GetUserOrders(userId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", err)
		return
	}

	if len(ordersInternal) == 0 {
		w.WriteHeader(http.StatusNoContent)
	} else {
		orders := make([]userOrder, 0, len(ordersInternal))
		for _, v := range ordersInternal {
			orders = append(orders, userOrder{
				Number:     v.Number,
				Status:     v.Status,
				Accrual:    v.Accrual,
				UploadedAt: v.UploadedAt,
			})
		}
		writeJSON(w, http.StatusOK, orders)
	}
}

// GetBalance обрабатывает GET /api/user/balance.
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	panic("не реализовано")
}

// Withdraw обрабатывает POST /api/user/balance/withdraw.
// 200 OK при успехе.
// 402 Payment Required при нехватке баллов.
// 422 Unprocessable Entity при неверном номере заказа.
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
	panic("не реализовано")
}

// GetWithdrawals обрабатывает GET /api/user/withdrawals.
// 200 OK с массивом или 204 No Content если списаний нет.
func (h *Handler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	panic("не реализовано")
}

// ExportStats обрабатывает POST /api/stats/export.
// Собирает статистику системы и записывает её в текстовый файл stats.txt
// в корне проекта. Возвращает 200 OK при успехе.
//
// Файл должен содержать:
//   - общее число зарегистрированных пользователей
//   - общее число заказов и их распределение по статусам
//   - суммарное количество начисленных баллов
//   - суммарное количество списанных баллов
//   - время генерации отчёта
//
// Для работы с файлами используйте пакет os (неделя 8).
func (h *Handler) ExportStats(w http.ResponseWriter, r *http.Request) {
	panic("не реализовано")
}

// ---------------------------------------------------------------------------
// Вспомогательная функция для работы с контекстом - предоставлена
// ---------------------------------------------------------------------------

type contextKey string

const CtxKeyUserID contextKey = "userID"

// UserIDFromContext извлекает ID аутентифицированного пользователя из контекста.
// Возвращает 0, false если значение отсутствует.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(CtxKeyUserID).(int64)
	return id, ok
}
