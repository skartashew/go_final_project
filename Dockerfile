# Используем образ golang для сборки приложения
FROM golang:1.24 AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Компилируем приложение для Linux
RUN GOOS=linux GOARCH=amd64 go build -o scheduler

# Создаём финальный образ на основе ubuntu
FROM ubuntu:latest

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем скомпилированный бинарник и папку web из стадии builder
COPY --from=builder /app/scheduler .
COPY --from=builder /app/web ./web

# Указываем порт, который будет использоваться
EXPOSE 7540

# Задаём переменные окружения по умолчанию (без пароля)
ENV TODO_PORT=7540
ENV TODO_DBFILE=/data/scheduler.db

# Команда для запуска сервера
CMD ["./scheduler"]