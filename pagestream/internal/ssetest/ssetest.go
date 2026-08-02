package ssetest

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type event struct {
	kind string
	data string
}

func PatchSignals(t testing.TB, body string) []map[string]any {
	t.Helper()
	var patches []map[string]any
	for _, event := range parseEvents(body) {
		if event.kind != "datastar-patch-signals" {
			continue
		}
		payload, err := signalsPayload(event.data)
		if err != nil {
			t.Fatal(err)
		}
		var patch map[string]any
		if err := json.Unmarshal([]byte(payload), &patch); err != nil {
			t.Fatalf("decode Datastar signal patch: %v", err)
		}
		patches = append(patches, patch)
	}
	return patches
}

func parseEvents(body string) []event {
	var events []event
	var current event
	var data []string
	flush := func() {
		if current.kind == "" && len(data) == 0 {
			return
		}
		current.data = strings.Join(data, "\n")
		events = append(events, current)
		current, data = event{}, nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if line == "" {
			flush()
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, ":") {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch name {
		case "event":
			current.kind = value
		case "data":
			data = append(data, value)
		}
	}
	flush()
	return events
}

func signalsPayload(data string) (string, error) {
	var payload []string
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "onlyIfMissing ") {
			continue
		}
		if !strings.HasPrefix(line, "signals ") {
			return "", fmt.Errorf("Datastar signal data %q is missing signals prefix", line)
		}
		payload = append(payload, strings.TrimPrefix(line, "signals "))
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("Datastar signal event has no payload")
	}
	return strings.Join(payload, "\n"), nil
}
