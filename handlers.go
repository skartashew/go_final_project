package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Task - структура для задачи, как она хранится в базе
type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// taskHandler - обработчик для маршрута /api/task
func taskHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getTask(w, r)
	case "POST":
		addTask(w, r)
	case "PUT":
		updateTask(w, r)
	case "DELETE":
		deleteTask(w, r)
	default:
		http.Error(w, `{"error":"Этот метод не работает"}`, http.StatusMethodNotAllowed)
	}
}

// addTask - добавляет новую задачу в базу
func addTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	var task struct {
		Date    string `json:"date"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
		Repeat  string `json:"repeat"`
	}

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, `{"error":"Ошибка в JSON"}`, http.StatusBadRequest)
		return
	}

	if task.Title == "" {
		http.Error(w, `{"error":"Заголовок обязателен"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	today := now.Format("20060102")
	if task.Date == "" {
		task.Date = today
	}

	dateParsed, err := time.Parse("20060102", task.Date)
	if err != nil {
		http.Error(w, `{"error":"Неправильная дата"}`, http.StatusBadRequest)
		return
	}

	// Если дата раньше today, заменяем на today
	if dateParsed.Before(now) {
		task.Date = today
	}

	// Проверяем правило повторения
	if task.Repeat != "" {
		_, err := NextDateSimple(now, task.Date, task.Repeat)
		if err != nil {
			http.Error(w, `{"error":"Ошибка в правиле повторения"}`, http.StatusBadRequest)
			return
		}
	}

	result, err := db.Exec("INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)",
		task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		http.Error(w, `{"error":"Не получилось добавить задачу"}`, http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, `{"error":"Не могу получить ID"}`, http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"id":"%d"}`, id)
}

// getTask - получает задачу по ID
func getTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}

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

	json.NewEncoder(w).Encode(task)
}

// updateTask - обновляет задачу
func updateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, `{"error":"Ошибка в JSON"}`, http.StatusBadRequest)
		return
	}

	if task.ID == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}
	if task.Title == "" {
		http.Error(w, `{"error":"Заголовок обязателен"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	today := now.Format("20060102")
	if task.Date == "" {
		task.Date = today
	}

	dateParsed, err := time.Parse("20060102", task.Date)
	if err != nil {
		http.Error(w, `{"error":"Неправильная дата"}`, http.StatusBadRequest)
		return
	}

	if dateParsed.Before(now) {
		task.Date = today
	}

	if task.Repeat != "" {
		_, err := NextDateSimple(now, task.Date, task.Repeat)
		if err != nil {
			http.Error(w, `{"error":"Ошибка в правиле повторения"}`, http.StatusBadRequest)
			return
		}
	}

	result, err := db.Exec("UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?",
		task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		http.Error(w, `{"error":"Ошибка обновления"}`, http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	}

	w.Write([]byte(`{}`))
}

// deleteTask - удаляет задачу по ID
func deleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM scheduler WHERE id = ?", id)
	if err != nil {
		http.Error(w, `{"error":"Ошибка удаления"}`, http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	}

	w.Write([]byte(`{}`))
}

// tasksHandler - возвращает список задач
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	search := r.URL.Query().Get("search")

	query := "SELECT id, date, title, comment, repeat FROM scheduler"
	var args []interface{}

	if search != "" {
		if parsedDate, err := time.Parse("02.01.2006", search); err == nil {
			query += " WHERE date = ?"
			args = append(args, parsedDate.Format("20060102"))
		} else {
			query += " WHERE title LIKE ? OR comment LIKE ?"
			searchPattern := "%" + search + "%"
			args = append(args, searchPattern, searchPattern)
		}
	}

	query += " ORDER BY date LIMIT 50"

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, `{"error":"Ошибка в базе"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

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

	if tasks == nil {
		tasks = []Task{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
}

// nextDateHandler - считает следующую дату для повторения
func nextDateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	nowStr := r.FormValue("now")
	dateStr := r.FormValue("date")
	repeat := r.FormValue("repeat")

	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		http.Error(w, `{"error":"Ошибка в параметре now"}`, http.StatusBadRequest)
		return
	}
	
	next, err := NextDateSimple(now, dateStr, repeat)
	if err != nil {
		http.Error(w, `{"error":"Ошибка в вычислении даты"}`, http.StatusBadRequest)
		return
	}

	fmt.Fprint(w, next)
}
