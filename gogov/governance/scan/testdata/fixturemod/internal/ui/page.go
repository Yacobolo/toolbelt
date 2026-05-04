package ui

import "example.com/fixture/internal/store"

func RenderCount() int {
	return store.CountTodos()
}
