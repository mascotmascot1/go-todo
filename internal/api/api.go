package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/mascotmascot1/go-todo/internal/config"
	"github.com/mascotmascot1/go-todo/internal/db"

	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	logger *log.Logger
	limits *config.Limits
	auth   *config.Auth
}

type response struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
	Token string `json:"token,omitempty"`
}

type tasksResponse struct {
	Tasks []*db.Task `json:"tasks"`
}

// NewHandlers creates new Handlers instance with given limits, auth and logger.
// It's used as a helper function to create handlers with required dependencies.
func NewHandlers(limits *config.Limits, auth *config.Auth, logger *log.Logger) *Handlers {
	return &Handlers{
		logger: logger,
		limits: limits,
		auth:   auth,
	}
}

// Init initializes handlers with given router and handlers instance.
// It sets up logging and size limit middlewares, then defines routes for
// signin, nextdate, tasks, task, update, delete and task done handlers.
// All routes inside the group are protected with authentication middleware.
func Init(r chi.Router, h *Handlers) {
	r.Use(h.withLogging)
	r.Use(h.withSizeLimit)

	r.Post("/api/signin", h.signInHandler)
	r.Get("/api/nextdate", h.nextDateHandler)

	r.Group(func(r chi.Router) {
		r.Use(h.withAuth)
		r.Get("/api/tasks", h.tasksHandler)
		r.Post("/api/task", h.addTaskHandler)
		r.Get("/api/task", h.taskHandler)
		r.Put("/api/task", h.updateHandler)
		r.Delete("/api/task", h.deleteTask)
		r.Post("/api/task/done", h.taskDoneHandler)
	})
}

// withLogging returns a middleware that logs each incoming request.
// It logs request method, URI and remote address.
func (h *Handlers) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.logger.Printf("Request: %s %s from %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// withSizeLimit returns a middleware that limits the size of each incoming request
// to the specified value in limits. It's intended to be used to prevent abuse and
// protect server from running out of memory.
func (h *Handlers) withSizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, h.limits.MaxUploadSize)
		next.ServeHTTP(w, r)
	})
}

// tasksHandler returns a list of tasks based on the given search string.
// It will return tasks that match the search string in either title or comment.
// If the search string is empty, it will return all tasks up to the limit set in the configuration.
// The response will be in JSON format and will contain a list of tasks under the key "tasks".
func (h *Handlers) tasksHandler(w http.ResponseWriter, r *http.Request) {
	search := r.FormValue("search")

	tasks, err := db.Tasks(h.limits.TasksLimit, search)
	if err != nil {
		h.failWithTaskError(w, "tasksHandler", err)
		return
	}
	h.writeJSON(w, tasksResponse{Tasks: tasks}, http.StatusOK)
}

// taskHandler returns a single task based on the given id.
// If the task doesn't exist, it will return an error with 404 status code.
// The response will be in JSON format and will contain the task under the key "task".
func (h *Handlers) taskHandler(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	task, err := db.GetTask(id)
	if err != nil {
		h.failWithTaskError(w, "taskHandler", err)
		return
	}

	h.writeJSON(w, task, http.StatusOK)
}

// updateHandler updates the task with the given id.
// The request body must contain the task in JSON format.
// If the task doesn't exist, it will return an error with 404 status code.
// If the task exists, it will update the task and return an empty response with 200 status code.
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func (h *Handlers) updateHandler(w http.ResponseWriter, r *http.Request) {
	caller := "updateHandler"

	content, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Printf("%s: failed to read body: %v\n", caller, err)
		h.writeJSON(w, response{Error: "failed to read request body"}, http.StatusBadRequest)
		return
	}

	var task db.Task
	if err := json.Unmarshal(content, &task); err != nil {
		h.logger.Printf("%s: json marshal error: %v\n", caller, err)
		h.writeJSON(w, response{Error: fmt.Sprintf("JSON deserialization failed: %v", err)}, http.StatusBadRequest)
		return
	}
	if err := validateTask(&task); err != nil {
		h.logger.Printf("%s: validation failed: %v\n", caller, err)
		h.writeJSON(w, response{Error: err.Error()}, http.StatusBadRequest)
		return
	}

	if err := db.UpdateTask(&task); err != nil {
		h.failWithTaskError(w, caller, err)
		return
	}

	h.writeJSON(w, struct{}{}, http.StatusOK)
}

// taskDoneHandler marks the task with the given id as done.
// If the task doesn't exist, it will return an error with 404 status code.
// If the task exists, it will update the task date based on its repeat field.
// If the task doesn't have a repeat field, it will delete the task instead.
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func (h *Handlers) taskDoneHandler(w http.ResponseWriter, r *http.Request) {
	caller := "taskDoneHandler"

	id := r.FormValue("id")
	task, err := db.GetTask(id)
	if err != nil {
		h.failWithTaskError(w, caller, err)
		return
	}

	if task.Repeat == "" {
		if err := db.DeleteTask(id); err != nil {
			h.failWithTaskError(w, caller, err)
			return
		}
		h.writeJSON(w, struct{}{}, http.StatusOK)
		return
	}

	nextDate, err := NextDate(time.Now(), task.Date, task.Repeat)
	if err != nil {
		h.logger.Printf("%s: failed to compute the new date: %v\n", caller, err)
		h.writeJSON(w, response{Error: fmt.Sprintf("failed to compute the new date: %v", err)}, http.StatusBadRequest)
		return
	}

	if err := db.UpdateDate(id, nextDate); err != nil {
		h.failWithTaskError(w, caller, err)
		return
	}

	h.writeJSON(w, struct{}{}, http.StatusOK)
}

// deleteTask deletes a task with the given id.
// If the task doesn't exist, it will return an error with 404 status code.
// If the task exists, it will delete the task and return an empty response with 200 status code.
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
func (h *Handlers) deleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if err := db.DeleteTask(id); err != nil {
		h.failWithTaskError(w, "deleteTask", err)
		return
	}

	h.writeJSON(w, struct{}{}, http.StatusOK)
}

// addTaskHandler adds a new task to the database.
// The request body must contain the task in JSON format.
// If the request body is invalid, it will return an error with 400 status code.
// If the request body is too large, it will return an error with 413 status code.
// If the task exists, it will return an error with 409 status code.
// If the task doesn't exist, it will add the task and return an empty response with 200 status code.
func (h *Handlers) addTaskHandler(w http.ResponseWriter, r *http.Request) {
	caller := "addTaskHandler"

	content, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Printf("%s: failed to read body: %v\n", caller, err)
		h.writeJSON(w, response{Error: "failed to read request body"}, http.StatusBadRequest)
		return
	}

	var task db.Task
	if err := json.Unmarshal(content, &task); err != nil {
		h.logger.Printf("%s: json marshal error: %v\n", caller, err)
		h.writeJSON(w, response{Error: fmt.Sprintf("JSON deserialization failed: %v", err)}, http.StatusBadRequest)
		return
	}
	if err := validateTask(&task); err != nil {
		h.logger.Printf("%s: validation failed: %v\n", caller, err)
		h.writeJSON(w, response{Error: err.Error()}, http.StatusBadRequest)
		return
	}

	id, err := db.AddTask(&task)
	if err != nil {
		h.failWithTaskError(w, caller, err)
		return
	}
	h.writeJSON(w, response{ID: strconv.FormatInt(id, 10)}, http.StatusOK)
}

// nextDateHandler returns the next date given a date and repeat rule.
// The request query must contain the following parameters:
// date: the date in the format "YYYY-MM-DD"
// repeat: the repeat rule in the format "d <number>|y <number>|w <number>,<number>,..."
// now: the current date in the format "YYYY-MM-DD", optional
// If the 'now' parameter is not provided, the current date will be used.
// If the 'now' parameter is invalid, it will return an error with 400 status code.
// If the 'date' or 'repeat' parameters are invalid, it will return an error with 400 status code.
// If the server failed to compute the next date, it will return an error with 400 status code.
// The response will be in plain text format and will contain the next date in the format "YYYY-MM-DD".
func (h *Handlers) nextDateHandler(w http.ResponseWriter, r *http.Request) {
	caller := "nextDayHandler"

	date := r.FormValue("date")
	repeat := r.FormValue("repeat")
	nowStr := r.FormValue("now")

	var (
		now time.Time
		err error
	)

	if nowStr == "" {
		now = time.Now()
	} else {
		now, err = time.Parse(db.DateLayoutDB, nowStr)
		if err != nil {
			h.logger.Printf("%s: invalid 'now' parameter: %v\n", caller, err)
			http.Error(w, fmt.Sprintf("invalid 'now' parameter: %v", err), http.StatusBadRequest)
			return
		}
	}

	newDate, err := NextDate(now, date, repeat)
	if err != nil {
		h.logger.Printf("%s: failed to compute the new date: %v\n", caller, err)
		http.Error(w, fmt.Sprintf("failed to compute the new date: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Add("Content-Type", "text/plain; charset=UTF-8")
	if _, err := w.Write([]byte(newDate)); err != nil {
		h.logger.Printf("failed to write response: %v\n", err)
	}
}

// failWithTaskError writes an error to the writer with the given status code and message.
// It also logs the error with the given caller string.
// If the error is db.ErrEmptyID, it will write the error with 400 status code.
// If the error is db.ErrTaskNotFound, it will write the error with 404 status code.
// Otherwise, it will write the error with 500 status code.
func (h *Handlers) failWithTaskError(w http.ResponseWriter, caller string, err error) {
	var (
		status = http.StatusInternalServerError
		msg    = "internal server error"
	)
	if errors.Is(err, db.ErrEmptyID) {
		status = http.StatusBadRequest
		msg = err.Error()
	}
	if errors.Is(err, db.ErrTaskNotFound) {
		status = http.StatusNotFound
		msg = err.Error()
	}

	h.logger.Printf("%s: %v\n", caller, err)
	h.writeJSON(w, response{Error: msg}, status)
}

// writeJSON writes the given data to the writer with the given status code.
// It assumes that the writer is already set up to write JSON data.
// If there is an error encoding the data, it logs the error.
// It does not return an error since the error is already logged.
// It also does not modify the writer's status code if there is an error encoding the data.
func (h *Handlers) writeJSON(w http.ResponseWriter, data any, code int) {
	w.Header().Add("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Printf("json encode error: %v\n", err)
	}
}

// validateTask validates a task by checking its title and date.
// It returns an error if the task's title is empty, or if the date is in the wrong format.
// It also updates the task's date if it's in the past and the task has a repeat field.
// If the task's date is in the past and it doesn't have a repeat field, it sets the task's date to today.
func validateTask(task *db.Task) error {
	if task.Title == "" {
		return fmt.Errorf("title is required")
	}

	now := midnight(time.Now())
	today := now.Format(db.DateLayoutDB)

	if task.Date == "" {
		task.Date = today
	}

	parsedDate, err := time.Parse(db.DateLayoutDB, task.Date)
	if err != nil {
		return fmt.Errorf("invalid date format")
	}

	var nextDate string
	if task.Repeat != "" {
		nextDate, err = NextDate(now, task.Date, task.Repeat)
		if err != nil {
			return err
		}
	}

	if parsedDate.Before(now) {
		if task.Repeat != "" {
			task.Date = nextDate
		} else {
			task.Date = today
		}
	}

	return nil
}
