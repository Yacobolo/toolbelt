// Package render provides gomponents rendering helpers for HTTP responses.
package render

import (
	"bytes"
	"fmt"
	g "maragu.dev/gomponents"
	"net/http"
)

// HTML renders a gomponents node as an HTML response.
func HTML(w http.ResponseWriter, node g.Node) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return node.Render(w)
}

// String renders a gomponents node to a string.
func String(node g.Node) (string, error) {
	var buf bytes.Buffer
	if err := node.Render(&buf); err != nil {
		return "", fmt.Errorf("render node: %w", err)
	}
	return buf.String(), nil
}
