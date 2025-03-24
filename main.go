package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3" // Драйвер для SQLite
)

const (
	defaultPort = ":7540"        // Порт по умолчанию для сервера
	webDir      = "./web"        // Папка со статическими файлами
	dbFileName  = "scheduler.db" // Имя файла базы данных
)

var db *sql.DB // Глобальная переменная для базы данных

// Главная функция программы
func main() {
	// Проверяем порт из переменной окружения
	port := os.Getenv("TODO_PORT")
	if port == "" {
		port = defaultPort // Если переменной нет, берём порт по умолчанию
	} else if port[0] != ':' {
		port = ":" + port // Добавляем двоеточие, если его нет
	}

	// Проверяем путь к базе данных
	dbFile := os.Getenv("TODO_DBFILE")
	if dbFile == "" {
		// Если путь не указан, берём текущую директорию
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal("Не могу найти рабочую директорию: ", err)
		}
		dbFile = filepath.Join(wd, dbFileName)
	}
	fmt.Println("Путь к базе данных:", dbFile)

	// Открываем базу данных
	var err error
	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatal("Ошибка открытия базы: ", err)
	}
	// Проверяем, что база работает
	if err = db.Ping(); err != nil {
		log.Fatal("Не могу подключиться к базе: ", err)
	}
	fmt.Println("База данных подключена!")

	// Проверяем, есть ли таблица scheduler
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='scheduler'").Scan(&tableName)
	if err == sql.ErrNoRows {
		// Если таблицы нет, создаём её
		_, err = db.Exec(`
            CREATE TABLE scheduler (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                date TEXT NOT NULL,
                title TEXT NOT NULL,
                comment TEXT,
                repeat TEXT CHECK (length(repeat) <= 128)
            );
            CREATE INDEX idx_date ON scheduler (date);
        `)
		if err != nil {
			log.Fatal("Ошибка создания таблицы: ", err)
		}
		fmt.Println("Таблица scheduler создана")
	} else if err != nil {
		log.Fatal("Ошибка проверки таблицы: ", err)
	} else {
		fmt.Println("Таблица scheduler уже есть")
	}

	// Настраиваем маршруты для HTTP
	http.Handle("/", http.FileServer(http.Dir(webDir))) // Статические файлы (без пароля)
	http.HandleFunc("/api/signin", signinHandler)       // Вход без проверки токена
	// Защищённые маршруты с проверкой токена
	http.HandleFunc("/api/nextdate", authMiddleware(nextDateHandler))
	http.HandleFunc("/api/task", authMiddleware(taskHandler))
	http.HandleFunc("/api/tasks", authMiddleware(tasksHandler))
	http.HandleFunc("/api/task/done", authMiddleware(doneTaskHandler))

	// Создаём сервер
	srv := &http.Server{
		Addr: port,
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		fmt.Println("Сервер запущен на http://localhost" + port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Ошибка сервера: ", err)
		}
	}()

	// Ждём сигнала для остановки (Ctrl+C)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	fmt.Println("\nОстанавливаем сервер...")

	// Даём серверу 5 секунд на завершение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("Ошибка при остановке: ", err)
	}
	if err := db.Close(); err != nil {
		log.Println("Ошибка закрытия базы: ", err)
	}
	fmt.Println("Сервер остановлен")
}
