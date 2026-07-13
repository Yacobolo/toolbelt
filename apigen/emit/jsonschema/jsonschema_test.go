package jsonschema

import (
	"encoding/json"
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/ir"
	"github.com/stretchr/testify/require"
)

func TestEmit_PreservesContractAndPropertyMetadata(t *testing.T) {
	doc := ir.Document{
		Info: ir.Info{Title: "LibreDash Signal Contracts", Version: "1.0.0"},
		Contracts: []ir.Contract{{
			Name:       "DashboardEnvelope",
			Schema:     ir.SchemaRef{Ref: "DashboardEnvelope"},
			Kind:       "ui-signal",
			Tags:       []string{"dashboard"},
			Extensions: map[string]any{"x-libredash-surface": "dashboard"},
		}},
		Schemas: map[string]ir.Schema{
			"DashboardEnvelope": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"visuals": {
						Schema:     ir.SchemaRef{Type: "object", AdditionalProperties: &ir.AdditionalProperties{Schema: &ir.SchemaRef{Ref: "DashboardVisual"}}},
						Extensions: map[string]any{"x-libredash-signal-key": "visuals"},
					},
				},
				Required: []string{"visuals"},
			},
			"DashboardVisual": {Type: "object", Properties: map[string]ir.SchemaProperty{"id": {Schema: ir.SchemaRef{Type: "string"}}}, Required: []string{"id"}},
		},
	}

	b, err := Emit(doc, Options{ID: "https://example.test/contracts.schema.json"})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))

	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", decoded["$schema"])
	require.Equal(t, "https://example.test/contracts.schema.json", decoded["$id"])
	require.Equal(t, "LibreDash Signal Contracts", decoded["title"])
	require.Contains(t, string(b), `"x-libredash-surface": "dashboard"`)
	require.Contains(t, string(b), `"x-libredash-signal-key": "visuals"`)
	require.Contains(t, string(b), `"#/$defs/DashboardEnvelope"`)
}
