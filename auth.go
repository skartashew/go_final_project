package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5" // Пакет для работы с JWT
)

// Claims - структура для данных в токене
type Claims struct {
	PasswordHash         string `json:"password_hash"` // Хэш пароля для проверки
	jwt.RegisteredClaims        // Стандартные поля JWT
}

// authMiddleware - проверяет токен перед выполнением запроса
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Смотрим наличие пароля
		pass := os.Getenv("TODO_PASSWORD")
		if len(pass) > 0 {
			var tokenStr string // JWT-токен из куки (переименовали jwt)

			// Получаем куку
			cookie, err := r.Cookie("token")
			if err == nil {
				tokenStr = cookie.Value // Если кука есть, берём значение
			}

			var valid bool // Флаг, чтобы понять, валиден ли токен
			// Здесь код для валидации и проверки JWT-токена
			if tokenStr != "" {
				// Создаём переменную для данных из токена
				claims := &Claims{}
				// Задаём секретный ключ (пока захардкодим)
				secretKey := "my_secret_key"

				// Проверяем токен
				token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(secretKey), nil // Ключ для проверки подписи
				})
				if err == nil && token.Valid {
					// Сравниваем хэш пароля из токена с текущим
					hash := fmt.Sprintf("%x", sha256.Sum256([]byte(pass)))
					if claims.PasswordHash == hash {
						valid = true // Токен валиден, если хэш совпадает
					}
				}
			}

			if !valid {
				// Возвращаем ошибку авторизации 401
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
		}
		// Если всё прошло, идём дальше
		next(w, r)
	})
}

// signinHandler - выдаёт токен при входе
func signinHandler(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем тип ответа
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Проверяем метод
	if r.Method != "POST" {
		http.Error(w, `{"error":"Только POST"}`, http.StatusMethodNotAllowed)
		return
	}

	// Структура для пароля из запроса
	var input struct {
		Password string `json:"password"`
	}
	// Читаем тело запроса
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"Ошибка в JSON"}`, http.StatusBadRequest)
		return
	}

	// Сравниваем пароли
	pass := os.Getenv("TODO_PASSWORD")
	if pass == "" || input.Password != pass {
		http.Error(w, `{"error":"Неправильный пароль"}`, http.StatusUnauthorized)
		return
	}

	// Создаём хэш пароля для токена
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(pass)))

	// Создаём токен
	claims := &Claims{
		PasswordHash: hash,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Токен на 24 часа
			IssuedAt:  jwt.NewNumericDate(time.Now()),                     // Время создания
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secretKey := "my_secret_key"
	tokenStr, err := token.SignedString([]byte(secretKey))
	if err != nil {
		http.Error(w, `{"error":"Не могу создать токен"}`, http.StatusInternalServerError)
		return
	}

	// Устанавливаем куку
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenStr,
		Expires: time.Now().Add(24 * time.Hour), // Куки тоже на 24 часа
	})

	// Отправляем токен в ответе
	fmt.Fprintf(w, `{"token":"%s"}`, tokenStr)
}
