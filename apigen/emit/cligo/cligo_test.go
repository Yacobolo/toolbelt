package cligo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Yacobolo/toolbelt/apigen/ir"
)

func TestEmit(t *testing.T) {
	t.Helper()
	doc := ir.Document{
		SchemaVersion: "v2",
		API:           ir.API{BasePath: "/v1"},
		Info:          ir.Info{Title: "t", Version: "1"},
		Schemas: map[string]ir.Schema{
			"CreateQueryRequest": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"sql": {Description: "SQL text to execute", Schema: ir.SchemaRef{Type: "string"}},
				},
				Required: []string{"sql"},
			},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/query",
				OperationID: "executeQuery",
				Summary:     "Execute a query",
				Description: "Runs SQL against the default catalog",
				Tags:        []string{"query"},
				Parameters: []ir.Parameter{
					{Name: "catalogName", In: "path", Required: true, Description: "Catalog to query", Schema: ir.SchemaRef{Type: "string"}},
				},
				RequestBody: &ir.RequestBody{Contents: []ir.BodyContent{{ContentType: "application/json", BodyKind: "json", Schema: &ir.SchemaRef{Ref: "CreateQueryRequest"}}}},
				Responses:   []ir.Response{{StatusCode: 200, Description: "ok"}},
				CLI: &ir.CLI{
					Command: []string{"query", "execute"},
				},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	require.Contains(t, string(b), "APIGeneratedCommandSpecs")
	require.Contains(t, string(b), "executeQuery")
	require.Contains(t, string(b), "Summary: \"Execute a query\"")
	require.Contains(t, string(b), "Description: \"Runs SQL against the default catalog\"")
	require.Contains(t, string(b), `Path: "/v1/query"`)
	require.Contains(t, string(b), "Parameters: []apigencobra.Param{{Name: \"catalogName\", In: \"path\", Type: \"string\", Description: \"Catalog to query\"")
	require.Contains(t, string(b), `RequestBody: &apigencobra.RequestBodySpec{Required: false, ContentType: "application/json", BodyKind: "json"`)
	require.Contains(t, string(b), "Fields: []apigencobra.Field{{Name: \"sql\", Type: \"string\", Description: \"SQL text to execute\"")
	require.Contains(t, string(b), "Command: []string{\"query\", \"execute\"}")
}

func TestEmit_IncludesMultipartPartSpecs(t *testing.T) {
	t.Helper()
	doc := ir.Document{
		SchemaVersion: "v2",
		API:           ir.API{BasePath: "/v1"},
		Info:          ir.Info{Title: "t", Version: "1"},
		Schemas: map[string]ir.Schema{
			"UploadMetadata": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"name": {Schema: ir.SchemaRef{Type: "string"}},
				},
				Required: []string{"name"},
			},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/artifacts",
				OperationID: "uploadArtifact",
				Summary:     "Upload artifact",
				RequestBody: &ir.RequestBody{
					Required: true,
					Contents: []ir.BodyContent{{
						ContentType: "multipart/form-data",
						BodyKind:    "multipart",
						Parts: []ir.MultipartPart{
							{Name: "metadata", WireName: "metadata", PartKind: "model", Required: true, ContentType: "application/json", BodyKind: "json", Schema: &ir.SchemaRef{Ref: "UploadMetadata"}},
							{Name: "artifact", WireName: "artifact", PartKind: "model", Required: true, ContentType: "application/octet-stream", BodyKind: "file", Filename: true, Schema: &ir.SchemaRef{Type: "string", Format: "binary"}},
							{Name: "tag", WireName: "tag", PartKind: "model", Repeated: true, ContentType: "text/plain", BodyKind: "text", Schema: &ir.SchemaRef{Type: "string"}},
						},
					}},
				},
				Responses: []ir.Response{{StatusCode: 201, Description: "created"}},
				CLI:       &ir.CLI{Command: []string{"artifact", "upload"}},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)
	require.Contains(t, content, `InputMode: "multipart"`)
	require.Contains(t, content, `Parts: []apigencobra.MultipartPartSpec{`)
	require.Contains(t, content, `{Name: "metadata", WireName: "metadata", PartKind: "model", Repeated: false, Required: true, ContentType: "application/json", BodyKind: "json", Filename: false, SchemaType: "object"}`)
	require.Contains(t, content, `{Name: "artifact", WireName: "artifact", PartKind: "model", Repeated: false, Required: true, ContentType: "application/octet-stream", BodyKind: "file", Filename: true, SchemaType: "string"}`)
	require.Contains(t, content, `{Name: "tag", WireName: "tag", PartKind: "model", Repeated: true, Required: false, ContentType: "text/plain", BodyKind: "text", Filename: false, SchemaType: "string"}`)
}

func TestEmit_RejectsInvalidCLI(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v2",
		API:           ir.API{BasePath: "/v1"},
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{
				Method:      "get",
				Path:        "/widgets",
				OperationID: "listWidgets",
				Summary:     "List widgets",
				Responses:   []ir.Response{{StatusCode: 200, Description: "ok"}},
				CLI:         &ir.CLI{},
			},
		},
	}

	_, err := Emit(doc, Options{})
	require.Error(t, err)
	require.ErrorContains(t, err, `cli.command is required`)
}
