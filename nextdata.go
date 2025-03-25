package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Константы для повторений
const (
	RepeatYearly = "y"
	RepeatDaily  = "d"
)

// NextDateSimple вычисляет следующую дату с учётом now
func NextDateSimple(now time.Time, startDate string, repeat string) (string, error) {
	log.Printf("NextDateSimple: now=%v, startDate=%s, repeat=%s\n", now, startDate, repeat)

	// Парсим исходную дату
	date, err := time.Parse("20060102", startDate)
	if err != nil {
		log.Printf("NextDateSimple: ошибка парсинга startDate=%s: %v\n", startDate, err)
		return "", fmt.Errorf("некорректная дата: %v", err)
	}
	log.Printf("NextDateSimple: распарсенная date=%v\n", date)

	// Проверяем правило повторения
	if repeat == "" {
		log.Println("NextDateSimple: правило повторения не указано")
		return "", fmt.Errorf("правило повторения не указано")
	}

	parts := strings.Fields(repeat)
	if len(parts) < 1 {
		log.Printf("NextDateSimple: неверный формат правила, parts=%v\n", parts)
		return "", fmt.Errorf("неверный формат правила")
	}
	log.Printf("NextDateSimple: parts=%v\n", parts)

	// Вычисляем следующую дату
	next := date
	switch parts[0] {
	case RepeatYearly:
		// Делаем один шаг на год
		next = next.AddDate(1, 0, 0)
		log.Printf("NextDateSimple: после первого шага 'y', next=%v\n", next)
		// Если результат меньше или равен now, продолжаем итерацию
		for next.Before(now) || next.Equal(now) {
			log.Printf("NextDateSimple: next=%v меньше или равен now=%v, делаем шаг\n", next, now)
			next = next.AddDate(1, 0, 0)
		}
		log.Printf("NextDateSimple: итоговая дата для 'y', next=%v\n", next)
		return next.Format("20060102"), nil

	case RepeatDaily:
		if len(parts) < 2 {
			log.Println("NextDateSimple: для 'd' не указаны дни")
			return "", fmt.Errorf("для 'd' нужно указать дни")
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil || days <= 0 || days > 400 {
			log.Printf("NextDateSimple: неправильное число дней, parts[1]=%s, err=%v\n", parts[1], err)
			return "", fmt.Errorf("неправильное число дней: %s", parts[1])
		}
		log.Printf("NextDateSimple: days=%d\n", days)
		// Делаем один шаг на days дней
		next = next.AddDate(0, 0, days)
		log.Printf("NextDateSimple: после первого шага 'd', next=%v\n", next)
		// Если результат меньше или равен now, продолжаем итерацию
		for next.Before(now) || next.Equal(now) {
			log.Printf("NextDateSimple: next=%v меньше или равен now=%v, делаем шаг\n", next, now)
			next = next.AddDate(0, 0, days)
		}
		log.Printf("NextDateSimple: итоговая дата для 'd', next=%v\n", next)
		return next.Format("20060102"), nil

	default:
		log.Printf("NextDateSimple: неподдерживаемое правило: %s\n", parts[0])
		return "", fmt.Errorf("неподдерживаемое правило: %s", parts[0])
	}
}

// doneTaskHandler обрабатывает запрос на выполнение задачи
func doneTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	log.Printf("doneTaskHandler: запрос %s %s\n", r.Method, r.URL.String())

	// Проверяем метод
	if r.Method != http.MethodPost {
		log.Println("doneTaskHandler: метод не POST")
		http.Error(w, `{"error":"Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	// Получаем ID
	id := r.URL.Query().Get("id")
	if id == "" {
		log.Println("doneTaskHandler: ID не указан")
		http.Error(w, `{"error":"ID не указан"}`, http.StatusBadRequest)
		return
	}
	log.Printf("doneTaskHandler: id=%s\n", id)

	// Запрашиваем задачу из базы
	var task struct {
		ID      string
		Date    string
		Title   string
		Comment string
		Repeat  string
	}
	err := db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", id).
		Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		log.Printf("doneTaskHandler: задача с id=%s не найдена\n", id)
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("doneTaskHandler: ошибка базы данных при запросе id=%s: %v\n", id, err)
		http.Error(w, `{"error":"Ошибка базы данных"}`, http.StatusInternalServerError)
		return
	}
	log.Printf("doneTaskHandler: найдена задача: %+v\n", task)

	// Обрабатываем в зависимости от repeat
	if task.Repeat == "" {
		// Удаляем задачу
		log.Printf("doneTaskHandler: repeat пустой, удаляем задачу id=%s\n", id)
		_, err := db.Exec("DELETE FROM scheduler WHERE id = ?", id)
		if err != nil {
			log.Printf("doneTaskHandler: ошибка удаления id=%s: %v\n", id, err)
			http.Error(w, `{"error":"Ошибка удаления"}`, http.StatusInternalServerError)
			return
		}
		log.Printf("doneTaskHandler: задача id=%s удалена\n", id)
		w.Write([]byte(`{}`))
		return
	}

	// Парсим текущую дату задачи
	currentDate, err := time.Parse("20060102", task.Date)
	if err != nil {
		log.Printf("doneTaskHandler: некорректная дата задачи id=%s: %v\n", id, err)
		http.Error(w, `{"error":"Некорректная дата задачи"}`, http.StatusInternalServerError)
		return
	}
	log.Printf("doneTaskHandler: текущая дата задачи id=%s: %v\n", id, currentDate)

	// Вычисляем следующую дату от текущей даты задачи (один шаг)
	parts := strings.Fields(task.Repeat)
	log.Printf("doneTaskHandler: правило повторения для id=%s: %v\n", id, parts)
	var nextDate time.Time
	switch parts[0] {
	case RepeatYearly:
		nextDate = currentDate.AddDate(1, 0, 0)
		log.Printf("doneTaskHandler: шаг 'y' для id=%s, nextDate=%v\n", id, nextDate)

	case RepeatDaily:
		if len(parts) < 2 {
			log.Println("doneTaskHandler: для 'd' не указаны дни")
			http.Error(w, `{"error":"Для 'd' нужно указать дни"}`, http.StatusBadRequest)
			return
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil || days <= 0 || days > 400 {
			log.Printf("doneTaskHandler: неправильное число дней для id=%s, parts[1]=%s: %v\n", id, parts[1], err)
			http.Error(w, `{"error":"Неправильное число дней"}`, http.StatusBadRequest)
			return
		}
		log.Printf("doneTaskHandler: days=%d для id=%s\n", days, id)
		nextDate = currentDate.AddDate(0, 0, days)
		log.Printf("doneTaskHandler: шаг 'd' для id=%s, nextDate=%v\n", id, nextDate)
	default:
		log.Printf("doneTaskHandler: неподдерживаемое правило для id=%s: %s\n", id, parts[0])
		http.Error(w, `{"error":"Неподдерживаемое правило"}`, http.StatusBadRequest)
		return
	}

	// Обновляем задачу
	log.Printf("doneTaskHandler: обновляем задачу id=%s с новой датой %s\n", id, nextDate.Format("20060102"))
	_, err = db.Exec("UPDATE scheduler SET date = ? WHERE id = ?", nextDate.Format("20060102"), id)
	if err != nil {
		log.Printf("doneTaskHandler: ошибка обновления id=%s: %v\n", id, err)
		http.Error(w, `{"error":"Ошибка обновления"}`, http.StatusInternalServerError)
		return
	}
	log.Printf("doneTaskHandler: задача id=%s успешно обновлена\n", id)

	w.Write([]byte(`{}`))
}

// isLeapYear проверяет, високосный ли год
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
