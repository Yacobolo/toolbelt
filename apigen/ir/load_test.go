package ir

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [
    {"method": "post", "path": "/v1/query", "operation_id": "executeQuery", "responses": [{"status_code": 200, "description": "ok"}]},
    {"method": "get", "path": "/healthz", "operation_id": "getHealth", "responses": [{"status_code": 200, "description": "ok"}]}
  ]
}`), 0o644))

	doc, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "getHealth", doc.Endpoints[0].OperationID)
}

func TestLoad_RejectsV2HTTPIR(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v2",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [
    {"method": "get", "path": "/healthz", "operation_id": "getHealth", "responses": [{"status_code": 200, "description": "ok"}]}
  ]
}`), 0o644))

	_, err := Load(path)
	require.ErrorContains(t, err, `unsupported schema_version "v2"`)
}

func TestLoad_AcceptsTypedEndpointTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/api/v1"},
  "info": {"title": "Tools", "version": "1.0.0"},
  "schemas": {
    "ListResponse": {
      "type": "object",
      "properties": {
        "items": {"schema": {"type": "array", "items": {"type": "string"}}},
        "page": {"schema": {"ref": "Page"}}
      },
      "required": ["items", "page"]
    },
    "Page": {
      "type": "object",
      "properties": {"nextCursor": {"schema": {"type": "string"}}}
    }
  },
  "endpoints": [{
    "method": "get",
    "path": "/workspaces/{workspace}/items",
    "operation_id": "listItems",
    "parameters": [
      {"name": "workspace", "in": "path", "required": true, "schema": {"type": "string"}},
      {"name": "limit", "in": "query", "schema": {"type": "integer", "format": "int32"}}
    ],
    "responses": [{"status_code": 200, "description": "ok", "contents": [{"content_type": "application/json", "body_kind": "json", "schema": {"ref": "ListResponse"}}]}],
    "tool": {
      "name": "list_items",
      "effect": "read",
      "confirmation": "never",
      "input": {"fields": [
        {"source": "path", "name": "workspace", "mode": "context", "context_key": "workspace"},
        {"source": "query", "name": "limit", "mode": "model", "default": 25}
      ]},
      "output": {
        "mode": "project",
        "select": [{"source": "/items", "count_as": "count"}],
        "cursor": {"source": "/page/nextCursor", "target": "nextCursor", "has_more_target": "hasMore"}
      },
      "metadata": {"x-product-surface": "catalog"}
    }
  }]
}`), 0o644))

	doc, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, doc.Endpoints[0].Tool)
	require.Equal(t, "list_items", doc.Endpoints[0].Tool.Name)
	require.Equal(t, "never", doc.Endpoints[0].Tool.Confirmation)
}

func TestValidate_AcceptsProjectionSelectionOnMapValues(t *testing.T) {
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		API:           API{BasePath: "/"},
		Info:          Info{Title: "Tools", Version: "1"},
		Schemas: map[string]Schema{
			"Visual": {
				Type:       "object",
				Properties: map[string]SchemaProperty{"id": {Schema: SchemaRef{Type: "string"}}},
				Required:   []string{"id"},
			},
			"Page": {
				Type: "object",
				Properties: map[string]SchemaProperty{
					"visuals": {Schema: SchemaRef{Type: "object", AdditionalProperties: &AdditionalProperties{Schema: &SchemaRef{Ref: "Visual"}}}},
				},
				Required: []string{"visuals"},
			},
		},
		Endpoints: []Endpoint{{
			Method: "get", Path: "/page", OperationID: "getPage",
			Responses: []Response{{StatusCode: 200, Description: "ok", Contents: []BodyContent{{ContentType: "application/json", BodyKind: "json", Schema: &SchemaRef{Ref: "Page"}}}}},
			Tool:      &Tool{Name: "get_page", Effect: "read", Output: ToolOutput{Mode: "project", Select: []ToolProjection{{Source: "/visuals", Select: []ToolProjection{{Source: "/id"}}}}}},
		}},
	}

	require.NoError(t, Validate(doc))
}

func TestLoad_RejectsLegacyAgentExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/"},
  "info": {"title": "Tools", "version": "1.0.0"},
  "endpoints": [{
    "method": "get", "path": "/items", "operation_id": "listItems",
    "responses": [{"status_code": 200, "description": "ok"}],
    "extensions": {"x-agent": {"name": "list_items"}}
  }]
}`), 0o644))

	_, err := Load(path)
	require.ErrorContains(t, err, `extension "x-agent" is reserved; use endpoint.tool`)
}

func TestLoad_AcceptsContractOnlyIR(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/"},
  "info": {"title": "Contracts", "version": "1.0.0"},
  "contracts": [{
    "name": "DashboardEnvelope",
    "schema": {"ref": "DashboardEnvelope"},
    "kind": "ui-signal",
    "tags": ["dashboard"],
    "extensions": {"x-libredash-surface": "dashboard"}
  }],
  "schemas": {
    "DashboardEnvelope": {
      "type": "object",
      "properties": {
        "page": {
          "schema": {"type": "string"},
          "extensions": {"x-libredash-signal-key": "page"}
        }
      },
      "required": ["page"]
    }
  }
}`), 0o644))

	doc, err := Load(path)
	require.NoError(t, err)
	require.Len(t, doc.Contracts, 1)
	require.Empty(t, doc.Endpoints)
}

func TestLoad_RejectsInvalidContractMetadata(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/"},
  "info": {"title": "Contracts", "version": "1.0.0"},
  "contracts": [{
    "name": "DashboardEnvelope",
    "schema": {"ref": "DashboardEnvelope"},
    "extensions": {"libredash": "dashboard"}
  }],
  "schemas": {
    "DashboardEnvelope": {"type": "object"}
  }
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, `extension "libredash" must start with "x-"`)
}

func TestLoad_InvalidVersion(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v0",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{"method": "get", "path": "/healthz", "operation_id": "getHealth", "responses": [{"status_code": 200, "description": "ok"}]}]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
}

func TestLoad_DuplicateOperation(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [
    {"method": "get", "path": "/healthz", "operation_id": "dup", "responses": [{"status_code": 200, "description": "ok"}]},
    {"method": "post", "path": "/v1/query", "operation_id": "dup", "responses": [{"status_code": 200, "description": "ok"}]}
  ]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
}

func TestLoad_NormalizesResponseHeaders(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets/{id}",
    "operation_id": "getWidget",
    "responses": [{
      "status_code": 429,
      "description": "rate limited",
      "headers": [
        {"name": "X-RateLimit-Reset", "schema": {"type": "integer", "format": "int64"}},
        {"name": "Retry-After", "schema": {"type": "integer", "format": "int32"}}
      ]
    }, {
      "status_code": 200,
      "description": "ok",
      "headers": [
        {"name": "X-RateLimit-Remaining", "schema": {"type": "integer", "format": "int32"}},
        {"name": "X-RateLimit-Limit", "schema": {"type": "integer", "format": "int32"}}
      ]
    }]
  }]
}`), 0o644))

	doc, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, 200, doc.Endpoints[0].Responses[0].StatusCode)
	require.Equal(t, "X-RateLimit-Limit", doc.Endpoints[0].Responses[0].Headers[0].Name)
	require.Equal(t, "X-RateLimit-Remaining", doc.Endpoints[0].Responses[0].Headers[1].Name)
	require.Equal(t, "Retry-After", doc.Endpoints[0].Responses[1].Headers[0].Name)
}

func TestLoad_RejectsDuplicateResponseHeaders(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets/{id}",
    "operation_id": "getWidget",
    "responses": [{
      "status_code": 200,
      "description": "ok",
      "headers": [
        {"name": "X-Test", "schema": {"type": "string"}},
        {"name": "x-test", "schema": {"type": "string"}}
      ]
    }]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate header")
}

func TestLoad_RejectsDuplicateResponseStatuses(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets/{id}",
    "operation_id": "getWidget",
    "responses": [{
      "status_code": 200,
      "description": "json"
    }, {
      "status_code": 200,
      "description": "binary"
    }]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate response status_code 200")
}

func TestLoad_AcceptsHeaderParameters(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets",
    "operation_id": "listWidgets",
    "parameters": [{"name": "Accept", "in": "header", "required": true, "schema": {"type": "string", "enum": ["application/json"]}}],
    "responses": [{"status_code": 200, "description": "ok"}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.NoError(t, err)
}

func TestLoad_RejectsUnsupportedParameterLocations(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets",
    "operation_id": "listWidgets",
    "parameters": [{"name": "session", "in": "cookie", "required": true, "schema": {"type": "string"}}],
    "responses": [{"status_code": 200, "description": "ok"}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, `unsupported parameter location "cookie"`)
}

func TestLoad_RejectsDuplicateContentTypes(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "post",
    "path": "/widgets",
    "operation_id": "createWidget",
    "request_body": {"contents": [
      {"content_type": "application/json", "body_kind": "json", "schema": {"type": "string"}},
      {"content_type": "application/json", "body_kind": "json", "schema": {"type": "string"}}
    ]},
    "responses": [{"status_code": 200, "description": "ok", "contents": [
      {"content_type": "application/json", "body_kind": "json", "schema": {"type": "string"}},
      {"content_type": "application/json", "body_kind": "json", "schema": {"type": "string"}}
    ]}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "duplicate content_type")
}

func TestLoad_ValidatesResponseShapeMetadata(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "post",
    "path": "/widgets",
    "operation_id": "createWidget",
    "responses": [{
      "status_code": 201,
      "description": "created",
      "extensions": {
        "x-apigen-response-shape": {
          "kind": "wrapped_json"
        }
      }
    }]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "wrapped_json body_type is required")
}

func TestLoad_AcceptsEndpointVendorExtensions(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets",
    "operation_id": "listWidgets",
    "extensions": {
      "x-downstream": {
        "enabled": true,
        "name": "list_workspace_assets",
        "risk": "read",
        "score": 1.5,
        "tags": ["workspace", "lineage"],
        "nested": {"nullable": null, "count": 3}
      }
    },
    "responses": [{"status_code": 200, "description": "ok"}]
  }]
}`), 0o644))

	doc, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, true, doc.Endpoints[0].Extensions["x-downstream"].(map[string]any)["enabled"])
}

func TestLoad_RejectsNonVendorEndpointExtensions(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets",
    "operation_id": "listWidgets",
    "extensions": {"agent": true},
    "responses": [{"status_code": 200, "description": "ok"}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, `extension "agent" must start with "x-"`)
}

func TestLoad_RejectsUnknownAPIGenEndpointExtensions(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets",
    "operation_id": "listWidgets",
    "extensions": {"x-apigen-tool": true},
    "responses": [{"status_code": 200, "description": "ok"}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, `unsupported APIGen-owned extension "x-apigen-tool"`)
}

func TestLoad_RejectsMalformedAPIGenEndpointExtensions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name      string
		extension map[string]any
		wantErr   string
	}{
		{
			name:      "manual must be boolean",
			extension: map[string]any{"x-apigen-manual": "true"},
			wantErr:   `x-apigen-manual must be boolean`,
		},
		{
			name:      "authz must be object",
			extension: map[string]any{"x-authz": "none"},
			wantErr:   `x-authz must be an object`,
		},
		{
			name:      "authz mode must be string",
			extension: map[string]any{"x-authz": map[string]any{"mode": true}},
			wantErr:   `x-authz.mode must be string`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			err := Validate(Document{
				SchemaVersion: "v4",
				API:           API{BasePath: "/v1"},
				Info:          Info{Title: "Duck", Version: "0.1.0"},
				Endpoints: []Endpoint{{
					Method:      "get",
					Path:        "/widgets",
					OperationID: "listWidgets",
					Extensions:  tc.extension,
					Responses:   []Response{{StatusCode: 200, Description: "ok"}},
				}},
			})
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestLoad_RejectsUnknownAPIGenResponseExtensions(t *testing.T) {
	t.Helper()

	err := Validate(Document{
		SchemaVersion: "v4",
		API:           API{BasePath: "/v1"},
		Info:          Info{Title: "Duck", Version: "0.1.0"},
		Endpoints: []Endpoint{{
			Method:      "get",
			Path:        "/widgets",
			OperationID: "listWidgets",
			Responses: []Response{{
				StatusCode:  200,
				Description: "ok",
				Extensions:  map[string]any{"x-apigen-other": true},
			}},
		}},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, `unsupported APIGen-owned extension "x-apigen-other"`)
}

func TestValidate_RejectsNonJSONCompatibleEndpointExtensionValues(t *testing.T) {
	t.Helper()

	err := Validate(Document{
		SchemaVersion: "v4",
		API:           API{BasePath: "/v1"},
		Info:          Info{Title: "Duck", Version: "0.1.0"},
		Endpoints: []Endpoint{{
			Method:      "get",
			Path:        "/widgets",
			OperationID: "listWidgets",
			Extensions:  map[string]any{"x-downstream": map[string]any{"score": math.Inf(1)}},
			Responses:   []Response{{StatusCode: 200, Description: "ok"}},
		}},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "number must be finite")
}

func TestLoad_RejectsMissingBasePath(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{"method": "get", "path": "/healthz", "operation_id": "getHealth", "responses": [{"status_code": 200, "description": "ok"}]}]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "api.base_path is required")
}

func TestLoad_RejectsUnknownSchemaRef(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "post",
    "path": "/widgets",
    "operation_id": "createWidget",
    "request_body": {"contents": [{"content_type": "application/json", "body_kind": "json", "schema": {"ref": "MissingRequest"}}]},
    "responses": [{"status_code": 201, "description": "created"}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, `references unknown schema "MissingRequest"`)
}

func TestValidate_AcceptsDiscriminatedComposition(t *testing.T) {
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		API:           API{BasePath: "/"},
		Info:          Info{Title: "Visual API", Version: "1.0.0"},
		Schemas: map[string]Schema{
			"Visual": {
				Type:  "union",
				OneOf: []SchemaRef{{Ref: "ChartVisual"}, {Ref: "TextVisual"}},
				Discriminator: &Discriminator{
					PropertyName: "shape",
					Mapping:      map[string]string{"chart": "ChartVisual", "text": "TextVisual"},
				},
			},
			"VisualBase":  {Type: "object", Properties: map[string]SchemaProperty{"shape": {Schema: SchemaRef{Type: "string"}}}, Required: []string{"shape"}},
			"ChartVisual": {Type: "object", Base: &SchemaRef{Ref: "VisualBase"}, Properties: map[string]SchemaProperty{"shape": {Schema: SchemaRef{Type: "string", Enum: []string{"chart"}}}}, Required: []string{"shape"}},
			"TextVisual":  {Type: "object", Base: &SchemaRef{Ref: "VisualBase"}, Properties: map[string]SchemaProperty{"shape": {Schema: SchemaRef{Type: "string", Enum: []string{"text"}}}}, Required: []string{"shape"}},
		},
		Contracts: []Contract{{Name: "visual", Schema: SchemaRef{Ref: "Visual"}}},
	}

	require.NoError(t, Validate(doc))
}

func TestValidate_RejectsInvalidDiscriminatorMapping(t *testing.T) {
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		API:           API{BasePath: "/"},
		Info:          Info{Title: "Visual API", Version: "1.0.0"},
		Schemas: map[string]Schema{
			"Visual": {
				Type:          "union",
				OneOf:         []SchemaRef{{Ref: "ChartVisual"}},
				Discriminator: &Discriminator{PropertyName: "shape", Mapping: map[string]string{"text": "Missing"}},
			},
			"ChartVisual": {Type: "object"},
		},
		Contracts: []Contract{{Name: "visual", Schema: SchemaRef{Ref: "Visual"}}},
	}

	err := Validate(doc)
	require.ErrorContains(t, err, "discriminator mapping")
	require.ErrorContains(t, err, "Missing")
}

func TestValidate_RejectsDiscriminatorWithoutMatchingVariantLiteral(t *testing.T) {
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		API:           API{BasePath: "/"},
		Info:          Info{Title: "Visual API", Version: "1.0.0"},
		Schemas: map[string]Schema{
			"Visual":      {Type: "union", OneOf: []SchemaRef{{Ref: "ChartVisual"}}, Discriminator: &Discriminator{PropertyName: "shape", Mapping: map[string]string{"chart": "ChartVisual"}}},
			"ChartVisual": {Type: "object", Properties: map[string]SchemaProperty{"shape": {Schema: SchemaRef{Type: "string", Enum: []string{"text"}}}}, Required: []string{"shape"}},
		},
		Contracts: []Contract{{Name: "visual", Schema: SchemaRef{Ref: "Visual"}}},
	}

	err := Validate(doc)
	require.ErrorContains(t, err, "matching literal property")
}

func TestValidate_AcceptsTransportErrorPolicy(t *testing.T) {
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		API:           API{BasePath: "/"},
		Info:          Info{Title: "API", Version: "1.0.0"},
		Schemas:       map[string]Schema{"Problem": {Type: "object"}},
		TransportErrors: &TransportErrors{
			Schema:      SchemaRef{Ref: "Problem"},
			ContentType: "application/problem+json",
			Failures: map[string]TransportFailure{
				"malformed_body": {StatusCode: 400, Code: "malformed_body", PublicDetail: "The request body is malformed."},
				"internal":       {StatusCode: 500, Code: "internal", PublicDetail: "Internal server error."},
			},
		},
		Endpoints: []Endpoint{{Method: "get", Path: "/", OperationID: "get", Responses: []Response{{StatusCode: 200, Description: "ok"}}}},
	}

	require.NoError(t, Validate(doc))
}

func TestValidate_RejectsEndpointResponseThatConflictsWithTransportPolicy(t *testing.T) {
	doc := Document{
		SchemaVersion: CurrentSchemaVersion,
		API:           API{BasePath: "/"},
		Info:          Info{Title: "API", Version: "1.0.0"},
		Schemas:       map[string]Schema{"Problem": {Type: "object"}, "Other": {Type: "object"}},
		TransportErrors: &TransportErrors{
			Schema:      SchemaRef{Ref: "Problem"},
			ContentType: "application/problem+json",
			Failures:    map[string]TransportFailure{"handler": {StatusCode: 500, Code: "internal", PublicDetail: "Internal server error."}},
		},
		Endpoints: []Endpoint{{Method: "get", Path: "/", OperationID: "get", Responses: []Response{{StatusCode: 500, Description: "error", Contents: []BodyContent{{ContentType: "application/json", BodyKind: "json", Schema: &SchemaRef{Ref: "Other"}}}}}}},
	}

	err := Validate(doc)
	require.ErrorContains(t, err, "conflicts with transport_errors")
}

func TestLoad_RejectsUnsupportedPathArrayParameter(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v4",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets/{ids}",
    "operation_id": "listWidgetsByIDs",
    "parameters": [{
      "name": "ids",
      "in": "path",
      "required": true,
      "schema": {"type": "array", "items": {"type": "string"}}
    }],
    "responses": [{"status_code": 200, "description": "ok"}]
  }]
}`), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "arrays are only supported in query parameters")
}
