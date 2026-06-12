// Пакет auth отвечает за генерацию и проверку токенов аутентификации.
// Токен - это случайная уникальная строка (например, UUID или hex-строка),
// которая однозначно связана с конкретным пользователем.
//
// Внутри пакета нужно хранить соответствие токен -> userID.
// Используйте для этого map с защитой от конкурентного доступа.
// Реализуйте этот пакет самостоятельно.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidToken возвращается, если токен не найден или недействителен.
var ErrInvalidToken = errors.New("недействительный токен")

// GenerateToken создаёт новый токен для пользователя с указанным ID
// и сохраняет связь токен -> userID внутри пакета.
func GenerateToken() string {
	token := uuid.New().String()

	return token
}

func Hash(item string) string {
	h := sha256.New()
	h.Write([]byte(item))
	hashBytes := h.Sum(nil)
	return hex.EncodeToString(hashBytes)
}
