package http

import "example.com/fixture/internal/services"

func HandlerGreeting() string {
	return services.Greet("browser")
}
