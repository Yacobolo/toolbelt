package services

import (
	"fmt"

	"example.com/fixture/internal/domain"
)

func RenderGreeting(greeter domain.Greeter, name string) string {
	return fmt.Sprintf("[%s]", greeter.Greet(name))
}

func Greet(name string) string {
	greeter := newServiceGreeter()
	return RenderGreeting(greeter, name)
}
