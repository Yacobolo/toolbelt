package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/Yacobolo/toolbelt/apigen/ir"
)

func TestEmitYAML(t *testing.T) {
	t.Helper()

	docIR := ir.Document{
		SchemaVersion: "v2",
		Info:          ir.Info{Title: "test", Version: "1.0.0"},
		Schemas: map[string]ir.Schema{
			"Item": {
				Type: "object",
				Example: map[string]any{
					"id": "item_123",
				},
				Properties: map[string]ir.SchemaProperty{
					"id": {Schema: ir.SchemaRef{Type: "string"}, Example: "item_123"},
				},
			},
			"Envelope": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"item": {Schema: ir.SchemaRef{Ref: "Item"}},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "get",
				Path:        "/items/{id}",
				OperationID: "getItem",
				Parameters: []ir.Parameter{
					{Name: "id", In: "path", Required: true, Schema: ir.SchemaRef{Type: "string"}, Example: "item_123"},
					{Name: "accept", In: "header", Required: true, Schema: ir.SchemaRef{Type: "string", Enum: []string{"application/json", "application/octet-stream"}}},
				},
				Extensions: map[string]any{
					"x-agent": map[string]any{
						"enabled": true,
						"name":    "get_item",
						"score":   1.5,
						"tags":    []any{"items", "read"},
						"nested":  map[string]any{"nullable": nil, "count": 3},
					},
				},
				Responses: []ir.Response{{
					StatusCode:  200,
					Description: "ok",
					Headers: []ir.Header{{
						Name:        "X-RateLimit-Remaining",
						Description: "Requests left in the current window.",
						Schema:      ir.SchemaRef{Type: "integer", Format: "int32"},
					}},
					Contents: []ir.BodyContent{{
						ContentType: "application/json",
						BodyKind:    "json",
						Schema:      &ir.SchemaRef{Ref: "Item"},
						Example: map[string]any{
							"id": "item_123",
						},
					}},
				}},
			},
		},
	}

	b, err := EmitYAML(docIR, Options{})
	require.NoError(t, err)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(b)
	require.NoError(t, err)
	require.Equal(t, "3.0.0", doc.OpenAPI)
	require.Equal(t, "getItem", doc.Paths.Value("/items/{id}").Get.OperationID)
	require.Equal(t, "item_123", doc.Paths.Value("/items/{id}").Get.Parameters[0].Value.Example)
	require.Equal(t, []any{"application/json", "application/octet-stream"}, doc.Paths.Value("/items/{id}").Get.Parameters[1].Value.Schema.Value.Enum)
	require.Equal(t, "item_123", doc.Components.Schemas["Item"].Value.Example.(map[string]any)["id"])
	require.Equal(t, true, doc.Paths.Value("/items/{id}").Get.Extensions["x-agent"].(map[string]any)["enabled"])
	require.Equal(t, []any{"items", "read"}, doc.Paths.Value("/items/{id}").Get.Extensions["x-agent"].(map[string]any)["tags"])
	require.Equal(t, nil, doc.Paths.Value("/items/{id}").Get.Extensions["x-agent"].(map[string]any)["nested"].(map[string]any)["nullable"])
	headers := doc.Paths.Value("/items/{id}").Get.Responses.Value("200").Value.Headers
	require.Contains(t, headers, "X-RateLimit-Remaining")
	require.Equal(t, openapi3.Types{"integer"}, *headers["X-RateLimit-Remaining"].Value.Schema.Value.Type)
	require.Equal(t, "item_123", doc.Paths.Value("/items/{id}").Get.Responses.Value("200").Value.Content.Get("application/json").Example.(map[string]any)["id"])

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal(b, &root))
	itemProperty := lookupYAMLMappingNode(&root, "components", "schemas", "Envelope", "properties", "item")
	require.NotNil(t, itemProperty)
	require.False(t, mappingNodeHasKey(itemProperty, "example"))
	require.Contains(t, string(b), "example:")
}

func TestEmitYAML_EmitsMultipleContentKinds(t *testing.T) {
	t.Helper()

	docIR := ir.Document{
		SchemaVersion: "v2",
		Info:          ir.Info{Title: "test", Version: "1.0.0"},
		Endpoints: []ir.Endpoint{{
			Method:      "get",
			Path:        "/artifact",
			OperationID: "getArtifact",
			Responses: []ir.Response{{
				StatusCode:  200,
				Description: "ok",
				Contents: []ir.BodyContent{
					{ContentType: "application/json", BodyKind: "json", Schema: &ir.SchemaRef{Type: "string"}},
					{ContentType: "application/octet-stream", BodyKind: "binary", Schema: &ir.SchemaRef{Type: "string", Format: "binary"}},
				},
			}},
		}},
	}

	b, err := EmitYAML(docIR, Options{})
	require.NoError(t, err)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(b)
	require.NoError(t, err)
	content := doc.Paths.Value("/artifact").Get.Responses.Value("200").Value.Content
	require.NotNil(t, content.Get("application/json"))
	require.NotNil(t, content.Get("application/octet-stream"))
	require.Equal(t, "binary", content.Get("application/octet-stream").Schema.Value.Format)
}

func TestEmitYAML_EmitsMultipartMetadata(t *testing.T) {
	t.Helper()

	docIR := ir.Document{
		SchemaVersion: "v2",
		Info:          ir.Info{Title: "test", Version: "1.0.0"},
		Schemas: map[string]ir.Schema{
			"Metadata": {Type: "object", Properties: map[string]ir.SchemaProperty{"name": {Schema: ir.SchemaRef{Type: "string"}}}},
		},
		Endpoints: []ir.Endpoint{{
			Method:      "post",
			Path:        "/artifact",
			OperationID: "uploadArtifact",
			RequestBody: &ir.RequestBody{Required: true, Contents: []ir.BodyContent{{
				ContentType: "multipart/form-data",
				BodyKind:    "multipart",
				Parts: []ir.MultipartPart{
					{Name: "metadata", WireName: "metadata", PartKind: "model", Required: true, ContentType: "application/json", BodyKind: "json", Schema: &ir.SchemaRef{Ref: "Metadata"}},
					{Name: "attachments", WireName: "attachments", PartKind: "model", Repeated: true, Required: true, ContentType: "application/octet-stream", BodyKind: "file", Filename: true, Schema: &ir.SchemaRef{Type: "string", Format: "binary"}},
				},
			}}},
			Responses: []ir.Response{{StatusCode: 204, Description: "ok"}},
		}},
	}

	b, err := EmitYAML(docIR, Options{})
	require.NoError(t, err)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(b)
	require.NoError(t, err)
	media := doc.Paths.Value("/artifact").Post.RequestBody.Value.Content.Get("multipart/form-data")
	require.NotNil(t, media)
	require.Equal(t, []string{"metadata", "attachments"}, media.Schema.Value.Required)
	require.Equal(t, openapi3.Types{"array"}, *media.Schema.Value.Properties["attachments"].Value.Type)
	require.Equal(t, "binary", media.Schema.Value.Properties["attachments"].Value.Items.Value.Format)
	require.Equal(t, "application/json", media.Encoding["metadata"].ContentType)
	require.Equal(t, "application/octet-stream", media.Encoding["attachments"].ContentType)
}

func TestEmitYAML_UsesAPIBasePathForVisibleRoutes(t *testing.T) {
	t.Helper()

	docIR := ir.Document{
		SchemaVersion: "v2",
		API:           ir.API{BasePath: "/v1"},
		Info:          ir.Info{Title: "test", Version: "1.0.0"},
		Endpoints: []ir.Endpoint{
			{
				Method:      "get",
				Path:        "/items/{id}",
				OperationID: "getItem",
				Responses:   []ir.Response{{StatusCode: 200, Description: "ok"}},
			},
		},
	}

	b, err := EmitYAML(docIR, Options{})
	require.NoError(t, err)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(b)
	require.NoError(t, err)
	require.NotNil(t, doc.Paths.Value("/v1/items/{id}"))
	require.Nil(t, doc.Paths.Value("/items/{id}"))
}

func lookupYAMLMappingNode(root *yaml.Node, path ...string) *yaml.Node {
	current := root
	if current.Kind == yaml.DocumentNode && len(current.Content) > 0 {
		current = current.Content[0]
	}
	for _, key := range path {
		if current == nil || current.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i < len(current.Content); i += 2 {
			if current.Content[i].Value == key {
				next = current.Content[i+1]
				break
			}
		}
		current = next
	}
	return current
}

func mappingNodeHasKey(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}
