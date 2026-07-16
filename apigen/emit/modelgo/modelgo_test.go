package modelgo

import (
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/ir"
	"github.com/stretchr/testify/require"
)

func TestEmit_GeneratesContractRootsAndDependencies(t *testing.T) {
	doc := contractDoc()

	b, err := Emit(doc, Options{PackageName: "contracts"})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "package contracts")
	require.Contains(t, content, "type DashboardEnvelope struct")
	require.Contains(t, content, "Page DashboardPageSignal `json:\"page\"`")
	require.Contains(t, content, "Visuals map[string]DashboardVisual `json:\"visuals\"`")
	require.Contains(t, content, "type DashboardVisual struct")
	require.Contains(t, content, "Data map[string]any `json:\"data\"`")
	require.Contains(t, content, "Note *string `json:\"note,omitempty\"`")
}

func TestEmit_GeneratesStrictDiscriminatedUnion(t *testing.T) {
	doc := ir.Document{
		Schemas: map[string]ir.Schema{
			"Visual":      {Type: "union", OneOf: []ir.SchemaRef{{Ref: "ChartVisual"}, {Ref: "TextVisual"}}, Discriminator: &ir.Discriminator{PropertyName: "shape", Mapping: map[string]string{"chart": "ChartVisual", "text": "TextVisual"}}},
			"VisualBase":  {Type: "object", Properties: map[string]ir.SchemaProperty{"shape": {Schema: ir.SchemaRef{Type: "string"}}}, Required: []string{"shape"}},
			"ChartVisual": {Type: "object", Base: &ir.SchemaRef{Ref: "VisualBase"}, Properties: map[string]ir.SchemaProperty{"shape": {Schema: ir.SchemaRef{Type: "string", Enum: []string{"chart"}}}, "points": {Schema: ir.SchemaRef{Type: "array", Items: &ir.SchemaRef{Type: "integer"}}}}, Required: []string{"shape", "points"}},
			"TextVisual":  {Type: "object", Base: &ir.SchemaRef{Ref: "VisualBase"}, Properties: map[string]ir.SchemaProperty{"shape": {Schema: ir.SchemaRef{Type: "string", Enum: []string{"text"}}}}, Required: []string{"shape"}},
		},
		Contracts: []ir.Contract{{Name: "visual", Schema: ir.SchemaRef{Ref: "Visual"}}},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)
	require.Contains(t, content, "type VisualVariant interface")
	require.Contains(t, content, "Value VisualVariant")
	require.Contains(t, content, "VisualBase")
	require.Contains(t, content, "func (value *Visual) UnmarshalJSON")
	require.Contains(t, content, "decoder.DisallowUnknownFields()")
	require.Contains(t, content, `case "chart":`)
	require.Contains(t, content, `if _, ok := fields["points"]; !ok`)
	require.Contains(t, content, `required property points is missing`)
}

func contractDoc() ir.Document {
	return ir.Document{
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
			"DashboardPageSignal": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"dashboardId": {Schema: ir.SchemaRef{Type: "string"}},
				},
				Required: []string{"dashboardId"},
			},
			"DashboardVisual": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"id":   {Schema: ir.SchemaRef{Type: "string"}},
					"data": {Schema: ir.SchemaRef{Type: "object", AdditionalProperties: &ir.AdditionalProperties{Schema: &ir.SchemaRef{}}}},
					"note": {Schema: ir.SchemaRef{Type: "string"}},
				},
				PropertyOrder: []string{"id", "data", "note"},
				Required:      []string{"id", "data"},
			},
		},
	}
}
