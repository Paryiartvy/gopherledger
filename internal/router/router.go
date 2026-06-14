// Пакет router собирает маршруты и middleware в единый HTTP-обработчик.
// Реализуйте этот пакет самостоятельно.
package router

import (
	"gopherledger/internal/middleware"
	"net/http"

	"gopherledger/internal/handler"
)

// New создаёт и возвращает HTTP-обработчик со всеми маршрутами.
//
// Публичные маршруты (без авторизации):
//
//	POST /api/user/register
//	POST /api/user/login
//
// Защищённые маршруты (требуют токен):
//
//	POST /api/user/orders
//	GET  /api/user/orders
//	GET  /api/user/balance
//	POST /api/user/balance/withdraw
//	GET  /api/user/withdrawals
//	POST /api/stats/export
func New(h *handler.Handler, validator middleware.Validator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Login)

	authMiddleware := middleware.AuthMiddleware(validator)
	mux.Handle("POST /api/user/orders", authMiddleware(http.HandlerFunc(h.CreateOrder)))
	mux.Handle("GET /api/user/orders", authMiddleware(http.HandlerFunc(h.GetOrders)))
	mux.Handle("GET /api/user/balance", authMiddleware(http.HandlerFunc(h.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", authMiddleware(http.HandlerFunc(h.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", authMiddleware(http.HandlerFunc(h.GetWithdrawals)))
	mux.Handle("POST /api/stats/export", authMiddleware(http.HandlerFunc(h.ExportStats)))

	return middleware.Logging(middleware.Recover(mux))
}
