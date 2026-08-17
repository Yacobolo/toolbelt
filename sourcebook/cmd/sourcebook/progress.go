package main

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Yacobolo/toolbelt/sourcebook/internal/sourcebook"
)

type commandProgress struct {
	mu        sync.Mutex
	output    io.Writer
	total     int
	complete  string
	completed int
	seen      map[string]struct{}
	writeErr  error
}

func newCommandProgress(output io.Writer, total int, complete string) *commandProgress {
	return &commandProgress{
		output:   output,
		total:    total,
		complete: complete,
		seen:     make(map[string]struct{}),
	}
}

func (p *commandProgress) Report(event sourcebook.UpdateEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writeErr != nil {
		return
	}

	var line string
	switch event.State {
	case sourcebook.UpdateRunning:
		phase := strings.TrimSpace(event.Phase)
		if phase == "" {
			return
		}
		key := event.Source + "\x00" + phase
		_, seen := p.seen[key]
		atEnd := event.Total > 0 && event.Current == event.Total

		if seen && !atEnd {
			return
		}
		p.seen[key] = struct{}{}
		if event.Total > 0 {
			phase = fmt.Sprintf("%s (%d/%d)", phase, event.Current, event.Total)
		}
		line = fmt.Sprintf("  %s: %s", event.Source, phase)
	case sourcebook.UpdateCompleted:
		p.completed++
		line = fmt.Sprintf("  [%d/%d] %s: %s", p.completed, p.total, event.Source, p.complete)
	case sourcebook.UpdateFailed:
		line = fmt.Sprintf("  %s: failed", event.Source)
	case sourcebook.UpdateCanceled:
		line = fmt.Sprintf("  %s: canceled", event.Source)
	case sourcebook.UpdateInstalling:
		line = "Installing refreshed sources..."
	default:
		return
	}

	if _, err := fmt.Fprintln(p.output, line); err != nil {
		p.writeErr = err
	}
}

func (p *commandProgress) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writeErr
}

func writeActionStart(output io.Writer, verb string, count int) error {
	noun := "sources"
	if count == 1 {
		noun = "source"
	}
	_, err := fmt.Fprintf(output, "%s %d %s...\n", verb, count, noun)
	return err
}

func writeActionSuccess(output io.Writer, count int, verb string) error {
	noun := "sources"
	if count == 1 {
		noun = "source"
	}
	_, err := fmt.Fprintf(output, "%s %d %s successfully.\n", verb, count, noun)
	return err
}
