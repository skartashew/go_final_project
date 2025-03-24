package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Task - структура для задачи, как она хранится в базе
type Task struct {
	ID      string `json:"id"`      // ID задачи как строка
	Date    string `json:"date"`    // Дата задачи
	Title   string `json:"title"`   // Название задачи
	Comment string `json:"comment"` // Комментарий (может быть пустым)
	Repeat  string `json:"repeat"`  // Правило повторения (может быть пустым)
}

// taskHandler - обработчик для маршрута /api/task
func taskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод запроса
	switch r.Method {
	case "GET":
		getTask(w, r) // Получаем задачу
	case "POST":
		addTask(w, r) // Добавляем задачу
	case "PUT":
		updateTask(w, r) // Обновляем задачу
	case "DELETE":
		deleteTask(w, r) // Удаляем задачу
	default:
		// Если метод неизвестный, возвращаем ошибку
		http.Error(w, `{"error":"Этот метод не работает"}`, http.StatusMethodNotAllowed)
	}
}

// addTask - добавляет новую задачу в базу
func addTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8") // Устанавливаем тип ответа

	// Структура для данных из запроса
	var task struct {
		Date    string `json:"date"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
		Repeat  string `json:"repeat"`
	}

	// Читаем JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, `{"error":"Ошибка в JSON"}`, http.StatusBadRequest)
		return
	}

	// Проверяем, что заголовок задачи есть
	if task.Title == "" {
		http.Error(w, `{"error":"Заголовок обязателен"}`, http.StatusBadRequest)
		return
	}

	// Если дата не указана, берём сегодняшнюю
	today := time.Now().Format("20060102")
	if task.Date == "" {
		task.Date = today
	}

	// Проверяем формат даты
	dateParsed, err := time.Parse("20060102", task.Date)
	if err != nil {
		http.Error(w, `{"error":"Неправильная дата"}`, http.StatusBadRequest)
		return
	}

	// Обрабатываем дату с учётом повторения
	if task.Repeat != "" {
		next, err := NextDate(time.Now(), task.Date, task.Repeat)
		if err != nil {
			http.Error(w, `{"error":"Ошибка в правиле повторения"}`, http.StatusBadRequest)
			return
		}
		task.Date = next // Устанавливаем следующую дату
	} else if dateParsed.Before(time.Now()) {
		task.Date = today // Если дата прошла и нет повторения, берём сегодня
	}

	// Добавляем задачу в базу
	result, err := db.Exec("INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)",
		task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		http.Error(w, `{"error":"Не получилось добавить задачу"}`, http.StatusInternalServerError)
		return
	}

	// Получаем ID новой задачи
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, `{"error":"Не могу получить ID"}`, http.StatusInternalServerError)
		return
	}

	// Отправляем ответ с ID
	fmt.Fprintf(w, `{"id":"%d"}`, id)
}

// getTask - получает задачу по ID
func getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Берём ID из параметров запроса
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}

	// Ищем задачу в базе
	var task Task
	err := db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", id).
		Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error":"Ошибка в базе"}`, http.StatusInternalServerError)
		return
	}

	// Отправляем задачу в JSON
	json.NewEncoder(w).Encode(task)
}

// updateTask - обновляет задачу
func updateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Читаем данные задачи из запроса
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, `{"error":"Ошибка в JSON"}`, http.StatusBadRequest)
		return
	}

	// Проверяем, что ID и заголовок есть
	if task.ID == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}
	if task.Title == "" {
		http.Error(w, `{"error":"Заголовок обязателен"}`, http.StatusBadRequest)
		return
	}

	// Если дата пустая, берём сегодняшнюю
	today := time.Now().Format("20060102")
	if task.Date == "" {
		task.Date = today
	}

	// Проверяем дату
	dateParsed, err := time.Parse("20060102", task.Date)
	if err != nil {
		http.Error(w, `{"error":"Неправильная дата"}`, http.StatusBadRequest)
		return
	}

	// Обрабатываем дату с учётом повторения
	if task.Repeat != "" {
		next, err := NextDate(time.Now(), task.Date, task.Repeat)
		if err != nil {
			http.Error(w, `{"error":"Ошибка в правиле повторения"}`, http.StatusBadRequest)
			return
		}
		task.Date = next
	} else if dateParsed.Before(time.Now()) {
		task.Date = today
	}

	// Обновляем задачу в базе
	result, err := db.Exec("UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?",
		task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		http.Error(w, `{"error":"Ошибка обновления"}`, http.StatusInternalServerError)
		return
	}

	// Проверяем, обновилась ли задача
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	}

	// Отправляем пустой ответ
	w.Write([]byte(`{}`))
}

// deleteTask - удаляет задачу по ID
func deleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Берём ID из запроса
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}

	// Удаляем задачу
	result, err := db.Exec("DELETE FROM scheduler WHERE id = ?", id)
	if err != nil {
		http.Error(w, `{"error":"Ошибка удаления"}`, http.StatusInternalServerError)
		return
	}

	// Проверяем, удалилась ли задача
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	}

	// Отправляем пустой ответ
	w.Write([]byte(`{}`))
}

// doneTaskHandler - завершает задачу
func doneTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Берём ID из запроса
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}

	// Ищем задачу
	var task Task
	err := db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", id).
		Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error":"Ошибка в базе"}`, http.StatusInternalServerError)
		return
	}

	// Если повторения нет, удаляем задачу
	if task.Repeat == "" {
		_, err := db.Exec("DELETE FROM scheduler WHERE id = ?", id)
		if err != nil {
			http.Error(w, `{"error":"Ошибка удаления"}`, http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{}`))
		return
	}

	// Вычисляем следующую дату
	nextDate, err := NextDate(time.Now(), task.Date, task.Repeat)
	if err != nil {
		http.Error(w, `{"error":"Ошибка с датой"}`, http.StatusBadRequest)
		return
	}

	// Обновляем дату в базе
	_, err = db.Exec("UPDATE scheduler SET date = ? WHERE id = ?", nextDate, id)
	if err != nil {
		http.Error(w, `{"error":"Ошибка обновления"}`, http.StatusInternalServerError)
		return
	}

	// Отправляем пустой ответ
	w.Write([]byte(`{}`))
}

// tasksHandler - возвращает список задач
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Берём параметр поиска
	search := r.URL.Query().Get("search")

	// Базовый запрос к базе
	query := "SELECT id, date, title, comment, repeat FROM scheduler"
	var args []interface{}

	// Если есть поиск
	if search != "" {
		// Проверяем, является ли search датой в формате 02.01.2006
		if parsedDate, err := time.Parse("02.01.2006", search); err == nil {
			query += " WHERE date = ?"
			args = append(args, parsedDate.Format("20060102"))
		} else {
			// Ищем по заголовку или комментарию
			query += " WHERE title LIKE ? OR comment LIKE ?"
			searchPattern := "%" + search + "%"
			args = append(args, searchPattern, searchPattern)
		}
	}

	// Добавляем сортировку и лимит
	query += " ORDER BY date LIMIT 50"

	// Выполняем запрос
	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, `{"error":"Ошибка в базе"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close() // Закрываем строки после использования

	// Собираем задачи в список
	var tasks []Task
	for rows.Next() {
		var task Task
		err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			http.Error(w, `{"error":"Ошибка чтения"}`, http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	// Если задач нет, возвращаем пустой список
	if tasks == nil {
		tasks = []Task{}
	}

	// Отправляем ответ
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
}

// nextDateHandler - считает следующую дату для повторения
func nextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем заголовок ответа
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// Берём параметры из запроса
	nowStr := r.FormValue("now")
	dateStr := r.FormValue("date")
	repeat := r.FormValue("repeat")

	// Преобразуем строку now в дату
	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		http.Error(w, `{"error":"Ошибка в параметре now"}`, http.StatusBadRequest)
		return
	}

	// Вычисляем следующую дату
	next, err := NextDate(now, dateStr, repeat)
	if err != nil {
		http.Error(w, `{"error":"Ошибка в вычислении даты"}`, http.StatusBadRequest)
		return
	}

	// Отправляем результат
	fmt.Fprint(w, next)
}

// NextDate - вычисляет следующую дату по правилу повторения
func NextDate(now time.Time, date string, repeat string) (string, error) {
	// Проверяем дату
	startDate, err := time.Parse("20060102", date)
	if err != nil {
		return "", fmt.Errorf("Неправильная дата: %v", err)
	}

	// Проверяем, указано ли правило
	if repeat == "" {
		return "", fmt.Errorf("Правило повторения не указано")
	}

	// Разбиваем правило на части
	parts := strings.Split(repeat, " ")
	if len(parts) == 0 {
		return "", fmt.Errorf("Неправильное правило")
	}

	// Обрабатываем разные типы правил
	switch parts[0] {
	case "d": // Повторение по дням
		if len(parts) != 2 {
			return "", fmt.Errorf("Для 'd' нужно указать дни")
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil || days <= 0 || days > 400 {
			return "", fmt.Errorf("Неправильное число дней: %s", parts[1])
		}
		next := startDate
		for next.Before(now) || next.Equal(now) {
			next = next.AddDate(0, 0, days)
		}
		return next.Format("20060102"), nil

	case "y": // Повторение по годам
		if len(parts) != 1 {
			return "", fmt.Errorf("Для 'y' параметры не нужны")
		}
		next := startDate
		for next.Before(now) || next.Equal(now) {
			next = next.AddDate(1, 0, 0)
		}
		return next.Format("20060102"), nil

	case "w": // Повторение по дням недели
		if len(parts) != 2 {
			return "", fmt.Errorf("Для 'w' нужны дни недели")
		}
		days := strings.Split(parts[1], ",")
		dayMap := make(map[int]bool)
		for _, d := range days {
			day, err := strconv.Atoi(d)
			if err != nil || day < 1 || day > 7 {
				return "", fmt.Errorf("Неправильный день недели: %s", d)
			}
			dayMap[day] = true
		}
		next := startDate
		for {
			if next.Before(now) || next.Equal(now) {
				next = next.AddDate(0, 0, 1)
				continue
			}
			weekday := int(next.Weekday())
			if weekday == 0 {
				weekday = 7 // Воскресенье как 7
			}
			if dayMap[weekday] {
				return next.Format("20060102"), nil
			}
			next = next.AddDate(0, 0, 1)
		}

	case "m": // Повторение по дням месяца
		if len(parts) < 2 || len(parts) > 3 {
			return "", fmt.Errorf("Для 'm' нужны дни и, возможно, месяцы")
		}
		days := strings.Split(parts[1], ",")
		dayMap := make(map[int]bool)
		for _, d := range days {
			day, err := strconv.Atoi(d)
			if err != nil || (day < -2 || day == 0 || day > 31) {
				return "", fmt.Errorf("Неправильный день месяца: %s", d)
			}
			dayMap[day] = true
		}
		var monthMap map[int]bool
		if len(parts) == 3 {
			months := strings.Split(parts[2], ",")
			monthMap = make(map[int]bool)
			for _, m := range months {
				month, err := strconv.Atoi(m)
				if err != nil || month < 1 || month > 12 {
					return "", fmt.Errorf("Неправильный месяц: %s", m)
				}
				monthMap[month] = true
			}
		}
		next := startDate
		for {
			if next.Before(now) || next.Equal(now) {
				next = next.AddDate(0, 0, 1)
				continue
			}
			day := next.Day()
			lastDay := next.AddDate(0, 1, -next.Day()).Day()
			if dayMap[day] || (dayMap[-1] && day == lastDay) || (dayMap[-2] && day == lastDay-1) {
				if monthMap == nil || monthMap[int(next.Month())] {
					return next.Format("20060102"), nil
				}
			}
			next = next.AddDate(0, 0, 1)
		}

	default:
		return "", fmt.Errorf("Неизвестное правило: %s", parts[0])
	}
}
