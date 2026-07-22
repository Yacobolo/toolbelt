package contractimport

import (
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/ir"
	"github.com/stretchr/testify/require"
)

func TestValidateRejectsMissingImportedModels(t *testing.T) {
	doc := ir.Document{
		Info: ir.Info{Namespace: "Consumer"},
		Schemas: map[string]ir.Schema{
			"Envelope": {Type: "object", Namespace: "Consumer", Properties: map[string]ir.SchemaProperty{
				"visual": {Schema: ir.SchemaRef{Ref: "MissingVisual"}},
			}},
		},
	}
	require.EqualError(t, Bindings{}.Validate(doc), `schema "Envelope" references unavailable exported model "MissingVisual"`)
}

func TestValidateRejectsExternalToLocalCycles(t *testing.T) {
	doc := ir.Document{
		Info: ir.Info{Namespace: "Consumer"},
		Schemas: map[string]ir.Schema{
			"Envelope": {Type: "object", Namespace: "Consumer"},
			"Visual": {Type: "object", Namespace: "Producer", Properties: map[string]ir.SchemaProperty{
				"owner": {Schema: ir.SchemaRef{Ref: "Envelope"}},
			}},
		},
	}
	imports := Bindings{"Producer": {GoPackage: "example.com/producer", GoAlias: "producer"}}
	require.EqualError(t, imports.Validate(doc), `contract import cycle: external schema "Visual" references local schema "Envelope"`)
}

func TestValidateRejectsImportedUnionVariants(t *testing.T) {
	doc := ir.Document{
		Info: ir.Info{Namespace: "Consumer"},
		Schemas: map[string]ir.Schema{
			"Visual":      {Type: "union", Namespace: "Consumer", OneOf: []ir.SchemaRef{{Ref: "PointVisual"}}, Discriminator: &ir.Discriminator{PropertyName: "kind", Mapping: map[string]string{"point": "PointVisual"}}},
			"PointVisual": {Type: "object", Namespace: "Producer"},
		},
	}
	imports := Bindings{"Producer": {GoPackage: "example.com/producer", GoAlias: "producer"}}
	require.EqualError(t, imports.Validate(doc), `local union "Visual" cannot use imported variant "PointVisual"`)
}
