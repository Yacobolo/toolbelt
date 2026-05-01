package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/example/apigen-example/internal/api"
)

func main() {
	addr := getenv("TODO_EXAMPLE_ADDR", "127.0.0.1:8081")

	router := chi.NewRouter()
	router.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		spec, err := api.GetEmbeddedOpenAPISpec()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	})
	api.RegisterAPIGenStrictRoutes(router, newServer())

	log.Printf("todo example listening on http://%s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	mu     sync.Mutex
	order  []string
	todos  map[string]api.Todo
	nextID int
}

func newServer() *server {
	s := &server{
		order: []string{"todo-1", "todo-2"},
		todos: map[string]api.Todo{
			"todo-1": {Id: "todo-1", Title: "write docs", Status: "open"},
			"todo-2": {Id: "todo-2", Title: "ship example", Status: "completed"},
		},
		nextID: 3,
	}
	return s
}

func (s *server) ListTodos(_ context.Context, request api.GenListTodosRequest) (api.GenListTodosResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	statusFilter := ""
	if request.Params.Status != nil {
		statusFilter = strings.TrimSpace(*request.Params.Status)
		if statusFilter != "" && statusFilter != "open" && statusFilter != "completed" {
			return api.GenListTodos400JSONResponse{
				GenBadRequestJSONResponse: api.GenBadRequestJSONResponse{
					Body: api.Error{Code: 400, Message: "status must be open or completed"},
				},
			}, nil
		}
	}

	response := api.ListTodosResponse{Data: make([]api.Todo, 0, len(s.order))}
	for _, id := range s.order {
		todo := s.todos[id]
		if statusFilter != "" && todo.Status != statusFilter {
			continue
		}
		response.Data = append(response.Data, todo)
	}

	return api.GenListTodos200JSONResponse{Body: response}, nil
}

func (s *server) CreateTodo(_ context.Context, request api.GenCreateTodoRequest) (api.GenCreateTodoResponse, error) {
	if request.Body == nil {
		return api.GenCreateTodo400JSONResponse{
			GenBadRequestJSONResponse: api.GenBadRequestJSONResponse{
				Body: api.Error{Code: 400, Message: "request body is required"},
			},
		}, nil
	}

	title := strings.TrimSpace(request.Body.Title)
	if title == "" {
		return api.GenCreateTodo400JSONResponse{
			GenBadRequestJSONResponse: api.GenBadRequestJSONResponse{
				Body: api.Error{Code: 400, Message: "title is required"},
			},
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := newTodoID(s.nextID)
	s.nextID++
	todo := api.Todo{Id: id, Title: title, Status: "open"}
	s.todos[id] = todo
	s.order = append(s.order, id)

	return api.GenCreateTodo201JSONResponse{Body: todo}, nil
}

func (s *server) GetTodo(_ context.Context, request api.GenGetTodoRequest) (api.GenGetTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	todo, ok := s.todos[request.TodoId]
	if !ok {
		return notFoundResponse(request.TodoId), nil
	}
	return api.GenGetTodo200JSONResponse{Body: todo}, nil
}

func (s *server) CompleteTodo(_ context.Context, request api.GenCompleteTodoRequest) (api.GenCompleteTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	todo, ok := s.todos[request.TodoId]
	if !ok {
		return api.GenCompleteTodo404JSONResponse{
			GenNotFoundJSONResponse: api.GenNotFoundJSONResponse{
				Body: api.Error{Code: 404, Message: "todo not found"},
			},
		}, nil
	}
	todo.Status = "completed"
	s.todos[request.TodoId] = todo
	return api.GenCompleteTodo200JSONResponse{Body: todo}, nil
}

func (s *server) DeleteTodo(_ context.Context, request api.GenDeleteTodoRequest) (api.GenDeleteTodoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.todos[request.TodoId]; !ok {
		return api.GenDeleteTodo404JSONResponse{
			GenNotFoundJSONResponse: api.GenNotFoundJSONResponse{
				Body: api.Error{Code: 404, Message: "todo not found"},
			},
		}, nil
	}

	delete(s.todos, request.TodoId)
	nextOrder := make([]string, 0, len(s.order))
	for _, id := range s.order {
		if id != request.TodoId {
			nextOrder = append(nextOrder, id)
		}
	}
	s.order = nextOrder

	return api.GenDeleteTodo204Response{}, nil
}

func getenv(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func newTodoID(nextID int) string {
	return fmt.Sprintf("todo-%d", nextID)
}

func notFoundResponse(todoID string) api.GenGetTodo404JSONResponse {
	message := "todo not found"
	if strings.TrimSpace(todoID) == "" {
		message = "todo id is required"
	}
	return api.GenGetTodo404JSONResponse{
		GenNotFoundJSONResponse: api.GenNotFoundJSONResponse{
			Body: api.Error{Code: 404, Message: message},
		},
	}
}
