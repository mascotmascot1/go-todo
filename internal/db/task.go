package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	DateLayoutSearch = "02.01.2006"
	DateLayoutDB     = "20060102"
)

var (
	ErrEmptyID      = errors.New("id mustn't be empty")
	ErrTaskNotFound = errors.New("task not found")
)

type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// Tasks returns a list of tasks based on the given search string.
// It will return tasks that match the search string in either title or comment.
// If the search string is empty, it will return all tasks up to the limit set in the configuration.
// The response will be in JSON format and will contain a list of tasks under the key "tasks".
func Tasks(limit int, search string) ([]*Task, error) {
	var (
		baseQuery = `SELECT id, date, title, comment, repeat FROM scheduler `
		rows      *sql.Rows
		errQuery  error
	)

	if search == "" {
		query := baseQuery + `ORDER BY date ASC LIMIT :limit`
		rows, errQuery = db.Query(query, sql.Named("limit", limit))
	} else {
		taskDate, err := time.Parse(DateLayoutSearch, search)
		if err != nil {
			search = "%" + search + "%"
			queryWord := baseQuery + `WHERE title LIKE :search OR comment LIKE :search ORDER BY date ASC LIMIT :limit`
			rows, errQuery = db.Query(queryWord, sql.Named("search", search), sql.Named("limit", limit))

		} else {
			queryDate := baseQuery + `WHERE date = :date ORDER BY date ASC LIMIT :limit`
			rows, errQuery = db.Query(queryDate, sql.Named("date", taskDate.Format(DateLayoutDB)), sql.Named("limit", limit))
		}
	}

	if errQuery != nil {
		return nil, fmt.Errorf("failed to select tasks: %w", errQuery)
	}
	defer rows.Close()

	tasks := make([]*Task, 0, limit)
	for rows.Next() {
		var task Task

		err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task while building task list: %w", err)
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows while building task list: %w", err)
	}
	return tasks, nil
}

// GetTask returns a single task based on the given id.
// If the task doesn't exist, it will return an error with 404 status code.
// The response will be in JSON format and will contain the task under the key "task".
func GetTask(id string) (*Task, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	var (
		task  Task
		query = `SELECT id, date, title, comment, repeat FROM scheduler WHERE id = :id`
	)
	row := db.QueryRow(query, sql.Named("id", id))
	if err := row.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return &task, nil
}

// UpdateTask updates the task with the given id.
// If the task doesn't exist, it will return an error with 404 status code.
// The response will be in JSON format and will contain the updated task under the key "task".
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func UpdateTask(task *Task) error {
	if task.ID == "" {
		return ErrEmptyID
	}

	query := `UPDATE scheduler SET date = :date, title = :title, comment = :comment, repeat = :repeat WHERE id = :id`

	res, err := db.Exec(query,
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat),
		sql.Named("date", task.Date),
		sql.Named("id", task.ID))
	if err != nil {
		return fmt.Errorf("failed to update task with id '%s': %w", task.ID, err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected while updating task: %w", err)
	}
	if count != 1 {
		return fmt.Errorf(`incorrect id for updating task '%s': %w`, task.ID, ErrTaskNotFound)
	}
	return nil
}

// UpdateDate updates the date of the task with the given id.
// If the task doesn't exist, it will return an error with 404 status code.
// The response will be in JSON format and will contain the updated task under the key "task".
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func UpdateDate(id, nextDate string) error {
	if id == "" {
		return ErrEmptyID
	}

	query := `UPDATE scheduler SET date = :date WHERE id = :id`

	res, err := db.Exec(query,
		sql.Named("date", nextDate),
		sql.Named("id", id))
	if err != nil {
		return fmt.Errorf("failed to update date for the task with id '%s': %w", id, err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected while updating date: %w", err)
	}
	if count != 1 {
		return fmt.Errorf(`incorrect id for updating date for the task '%s': %w`, id, ErrTaskNotFound)
	}
	return nil
}

// DeleteTask deletes a task with the given id.
// If the task doesn't exist, it will return an error with 404 status code.
// The response will be in JSON format and will contain an empty response with 200 status code.
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func DeleteTask(id string) error {
	if id == "" {
		return ErrEmptyID
	}

	query := `DELETE FROM scheduler WHERE id = :id`
	res, err := db.Exec(query, sql.Named("id", id))
	if err != nil {
		return fmt.Errorf("failed to delete task with id '%s': %w", id, err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected while deleting: %w", err)
	}
	if count != 1 {
		return fmt.Errorf(`incorrect id for deleting task '%s': %w`, id, ErrTaskNotFound)
	}
	return nil
}

// AddTask adds a new task to the database.
// It returns the id of the newly inserted task.
// If the task already exists, it will return an error with 409 status code.
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func AddTask(task *Task) (int64, error) {
	query := `INSERT INTO scheduler (date, title, comment, repeat) 
		VALUES (:date, :title, :comment, :repeat)`

	res, err := db.Exec(query,
		sql.Named("date", task.Date),
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat))
	if err != nil {
		return 0, fmt.Errorf("failed to add task with title '%s': %w", task.Title, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}
