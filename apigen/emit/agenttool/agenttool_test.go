package agenttool

import (
	"encoding/json"
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/ir"
	runtime "github.com/Yacobolo/toolbelt/apigen/runtime/agenttool"
	"github.com/stretchr/testify/require"
)

func TestBuildCompilesEndpointToolContract(t *testing.T) {
	doc := ir.Document{
		SchemaVersion: ir.CurrentSchemaVersion,
		API:           ir.API{BasePath: "/api/v1"},
		Info:          ir.Info{Title: "Tools", Version: "1"},
		Schemas: map[string]ir.Schema{
			"Item": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"id":     {Schema: ir.SchemaRef{Type: "string"}},
					"secret": {Schema: ir.SchemaRef{Type: "string"}},
				},
				Required: []string{"id"},
			},
			"ListResponse": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"items": {Schema: ir.SchemaRef{Type: "array", Items: &ir.SchemaRef{Ref: "Item"}}},
				},
				Required: []string{"items"},
			},
		},
		Endpoints: []ir.Endpoint{{
			Method: "get", Path: "/workspaces/{workspace}/items", OperationID: "listItems", Summary: "List items",
			Parameters: []ir.Parameter{
				{Name: "workspace", In: "path", Required: true, Schema: ir.SchemaRef{Type: "string"}},
				{Name: "limit", In: "query", Schema: ir.SchemaRef{Type: "integer", Format: "int32"}},
			},
			Responses: []ir.Response{{
				StatusCode:  200,
				Description: "ok",
				Contents:    []ir.BodyContent{{ContentType: "application/json", BodyKind: "json", Schema: &ir.SchemaRef{Ref: "ListResponse"}}},
			}},
			Tool: &ir.Tool{
				Name: "list_items", Effect: "read", Confirmation: "never",
				Input: &ir.ToolInput{Fields: []ir.ToolInputField{
					{Source: "path", Name: "workspace", Mode: "context", ContextKey: "workspace"},
					{Source: "query", Name: "limit", Default: 25},
				}},
				Output: ir.ToolOutput{Mode: "project", Select: []ir.ToolProjection{{Source: "/items", CountAs: "count", Select: []ir.ToolProjection{{Source: "/id"}}}}},
			},
		}},
	}

	contracts, err := Build(doc)
	require.NoError(t, err)
	contract := contracts["list_items"]
	require.Equal(t, "/api/v1/workspaces/{workspace}/items", contract.Path)
	require.Equal(t, runtime.EffectRead, contract.Effect)
	require.Equal(t, "context", contract.Bindings[0].Mode)
	require.Equal(t, "workspace", contract.Bindings[0].ContextKey)
	require.Equal(t, "limit", contract.Bindings[1].Argument)
	require.EqualValues(t, 25, contract.Bindings[1].Default)
	require.Equal(t, "array", contract.Output.Select[0].Kind)
	require.Equal(t, "value", contract.Output.Select[0].Select[0].Kind)

	var inputSchema map[string]any
	require.NoError(t, json.Unmarshal(contract.InputSchema, &inputSchema))
	properties := inputSchema["properties"].(map[string]any)
	require.NotContains(t, properties, "workspace")
	require.Contains(t, properties, "limit")
	require.NotContains(t, properties["limit"].(map[string]any), "format")
	require.NotContains(t, properties["limit"].(map[string]any), "default")
	require.Equal(t, false, inputSchema["additionalProperties"])
}
