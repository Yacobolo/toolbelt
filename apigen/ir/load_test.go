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
  "schema_version": "v2",
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
  "schema_version": "v2",
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
  "schema_version": "v2",
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
  "schema_version": "v2",
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
  "schema_version": "v2",
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

func TestLoad_ValidatesResponseShapeMetadata(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v2",
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
  "schema_version": "v2",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0"},
  "endpoints": [{
    "method": "get",
    "path": "/widgets",
    "operation_id": "listWidgets",
    "extensions": {
      "x-agent": {
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
	require.Equal(t, true, doc.Endpoints[0].Extensions["x-agent"].(map[string]any)["enabled"])
}

func TestLoad_RejectsNonVendorEndpointExtensions(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v2",
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
  "schema_version": "v2",
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
				SchemaVersion: "v2",
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
		SchemaVersion: "v2",
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
		SchemaVersion: "v2",
		API:           API{BasePath: "/v1"},
		Info:          Info{Title: "Duck", Version: "0.1.0"},
		Endpoints: []Endpoint{{
			Method:      "get",
			Path:        "/widgets",
			OperationID: "listWidgets",
			Extensions:  map[string]any{"x-agent": map[string]any{"score": math.Inf(1)}},
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
  "schema_version": "v2",
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
  "schema_version": "v2",
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

func TestLoad_RejectsUnsupportedPathArrayParameter(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(path, []byte(`{
  "schema_version": "v2",
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
