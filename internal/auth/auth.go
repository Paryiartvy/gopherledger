// Пакет auth отвечает за генерацию и проверку токенов аутентификации.
// Токен - это случайная уникальная строка (например, UUID или hex-строка),
// которая однозначно связана с конкретным пользователем.
//
// Внутри пакета нужно хранить соответствие токен -> userID.
// Используйте для этого map с защитой от конкурентного доступа.
// Реализуйте этот пакет самостоятельно.
package auth

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

var mu = sync.RWMutex{}
var userIDByToken = make(map[string]int64)
var tokenByUserID = make(map[int64]string)

// ErrInvalidToken возвращается, если токен не найден или недействителен.
var ErrInvalidToken = errors.New("недействительный токен")

// GenerateToken создаёт новый токен для пользователя с указанным ID
// и сохраняет связь токен -> userID внутри пакета.
func GenerateToken(userID int64) (string, error) {
	token := uuid.New().String()

	mu.Lock()
	defer mu.Unlock()
	userIDByToken[token] = userID
	delete(userIDByToken, tokenByUserID[userID])
	tokenByUserID[userID] = token

	return token, nil
}

// ValidateToken проверяет токен и возвращает ID пользователя.
// Возвращает ErrInvalidToken если токен не найден.
func ValidateToken(token string) (int64, error) {
	mu.RLock()
	defer mu.RUnlock()

	id, ok := userIDByToken[token]
	if !ok {
		return 0, ErrInvalidToken
	}
	if t, ok := tokenByUserID[id]; ok && t != token {
		return 0, ErrInvalidToken
	}

	return id, nil
}
