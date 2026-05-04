package services

import "testing"

func TestGreet(t *testing.T) {
	if got := Greet("team"); got != "[hi team]" {
		t.Fatalf("Greet() = %q", got)
	}
}
