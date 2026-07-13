package modelts

import (
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/ir"
	"github.com/stretchr/testify/require"
)

func TestEmit_GeneratesContractInterfaces(t *testing.T) {
	doc := ir.Document{
		Contracts: []ir.Contract{{Name: "DashboardEnvelope", Schema: ir.SchemaRef{Ref: "DashboardEnvelope"}}},
		Schemas: map[string]ir.Schema{
			"DashboardEnvelope": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"page":    {Schema: ir.SchemaRef{Ref: "DashboardPageSignal"}},
					"visuals": {Schema: ir.SchemaRef{Type: "object", AdditionalProperties: &ir.AdditionalProperties{Schema: &ir.SchemaRef{Ref: "DashboardVisual"}}}},
				},
				PropertyOrder: []string{"page", "visuals"},
				Required:      []string{"page", "visuals"},
			},
			"DashboardPageSignal": {Type: "object", Properties: map[string]ir.SchemaProperty{"pageId": {Schema: ir.SchemaRef{Type: "string"}}}, Required: []string{"pageId"}},
			"DashboardVisual": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"id":   {Schema: ir.SchemaRef{Type: "string"}},
					"data": {Schema: ir.SchemaRef{Type: "object", AdditionalProperties: &ir.AdditionalProperties{Schema: &ir.SchemaRef{}}}},
					"note": {Schema: ir.SchemaRef{Type: "string"}},
				},
				Required: []string{"id", "data"},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "export interface DashboardEnvelope")
	require.Contains(t, content, "page: DashboardPageSignal")
	require.Contains(t, content, "visuals: Record<string, DashboardVisual>")
	require.Contains(t, content, "data: Record<string, unknown>")
	require.Contains(t, content, "note?: string")
	require.Contains(t, content, "export type DataContractEnvelope = DashboardEnvelope")
}
