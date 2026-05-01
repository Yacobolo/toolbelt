package servergo

import (
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/ir"
	"github.com/stretchr/testify/require"
)

func TestEmit(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{Method: "get", Path: "/healthz", OperationID: "getHealth", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)
	require.Contains(t, content, "type GenServerInterface interface")
	require.Contains(t, content, "RegisterAPIGenRoutes")
	require.Contains(t, content, "HandleAPIGen")
	require.Contains(t, content, "type GenOperationDispatcher interface")
	require.Contains(t, content, "DispatchAPIGenOperation")
	require.NotContains(t, content, "*ServerInterfaceWrapper")
	require.Contains(t, content, "apigenchi.RegisterRoutes(router, []apigenchi.Route{")
	require.Contains(t, content, "{Method: \"GET\", Path: \"/healthz\", OperationID: \"getHealth\"}")
	require.Contains(t, content, "func RegisterAPIGenRoutes(router apigenchi.Router, server GenServerInterface)")
	require.Contains(t, content, "func RegisterAPIGenStrictRoutes(router apigenchi.Router, handler GenStrictServerInterface)")
	require.Contains(t, content, "func DispatchAPIGenOperation(operationID string, dispatcher GenOperationDispatcher")
	require.NotContains(t, content, "\"github.com/oapi-codegen/runtime\"")
	require.NotContains(t, content, "\"reflect\"")
	require.Contains(t, content, "type genStrictAdapter struct")
	require.Contains(t, content, "func (a genStrictAdapter) HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request)")
	require.Contains(t, content, "type genStrictBridge struct")
	require.Contains(t, content, "type GenStrictServerInterface interface")
	require.Contains(t, content, "func DispatchAPIGenStrictOperation(operationID string, handler GenStrictServerInterface")
	require.Contains(t, content, "type GenOperationContract struct")
	require.Contains(t, content, "var genOperationContracts = map[string]GenOperationContract{")
	require.Contains(t, content, "func GetAPIGenOperationContracts() map[string]GenOperationContract")
	require.Contains(t, content, "func GetAPIGenOperationContract(operationID string) (GenOperationContract, bool)")
	require.Contains(t, content, "func APIGenOperationAllowsStatus(operationID string, statusCode int) bool")
}

func TestEmit_UsesIRPathAsIs(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{Method: "post", Path: "/query", OperationID: "executeQuery", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)
	require.Contains(t, content, "{Method: \"POST\", Path: \"/query\", OperationID: \"executeQuery\"}")
	require.NotContains(t, content, "{Method: \"POST\", Path: \"/v1/query\", OperationID: \"executeQuery\"}")
}

func TestEmit_UsesAPIBasePathForRoutes(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		API:           ir.API{BasePath: "/v1"},
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{Method: "post", Path: "/query", OperationID: "executeQuery", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)
	require.Contains(t, content, "{Method: \"POST\", Path: \"/v1/query\", OperationID: \"executeQuery\"}")
}

func TestValidateOperationIDs(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{Method: "get", Path: "/a", OperationID: "create-user", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
			{Method: "post", Path: "/b", OperationID: "create_user", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
		},
	}

	err := ValidateOperationIDs(doc)
	require.Error(t, err)
}

func TestEmit_DispatchParityAndHealthHandling(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{Method: "get", Path: "/healthz", OperationID: "getHealth", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
			{Method: "post", Path: "/query", OperationID: "executeQuery", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
			{Method: "get", Path: "/groups", OperationID: "listGroups", Responses: []ir.Response{{StatusCode: 200, Description: "ok"}}},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "{Method: \"GET\", Path: \"/healthz\", OperationID: \"getHealth\"}")
	require.Contains(t, content, "{Method: \"POST\", Path: \"/query\", OperationID: \"executeQuery\"}")
	require.Contains(t, content, "{Method: \"GET\", Path: \"/groups\", OperationID: \"listGroups\"}")
	require.Contains(t, content, "}, server.HandleAPIGen)")

	require.Contains(t, content, "ExecuteQuery(w http.ResponseWriter, r *http.Request)")
	require.Contains(t, content, "ListGroups(w http.ResponseWriter, r *http.Request)")
	require.NotContains(t, content, "GetHealth(w http.ResponseWriter, r *http.Request)")

	require.Contains(t, content, "case \"executeQuery\":")
	require.Contains(t, content, "dispatcher.ExecuteQuery(w, r)")
	require.Contains(t, content, "case \"listGroups\":")
	require.Contains(t, content, "dispatcher.ListGroups(w, r)")
	require.Contains(t, content, "case \"getHealth\":")
	require.Contains(t, content, "w.Header().Set(\"Content-Type\", \"application/json\")")
	require.Contains(t, content, "_ = json.NewEncoder(w).Encode(map[string]string{\"status\": \"ok\"})")
	require.NotContains(t, content, "dispatcher.GetHealth(w, r)")
}

func TestEmit_OperationContractsIncludeManualAndBodyMetadata(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/auth/local-login",
				OperationID: "localLogin",
				Tags:        []string{"Auth"},
				RequestBody: &ir.RequestBody{Required: true, Schema: ir.SchemaRef{Ref: "LoginRequest"}},
				Responses: []ir.Response{
					{StatusCode: 200, Description: "ok"},
					{StatusCode: 401, Description: "unauthorized"},
				},
				Extensions: map[string]any{
					"x-apigen-manual": true,
					"x-authz":         map[string]any{"mode": "none"},
				},
			},
		},
		Schemas: map[string]ir.Schema{
			"LoginRequest": {Type: "object"},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)
	require.Contains(t, content, `"localLogin": {OperationID: "localLogin", Method: "POST", Path: "/auth/local-login"`)
	require.Contains(t, content, `DocumentedStatusCodes: []int{200, 400, 401, 403, 404, 409, 429, 500, 502}`)
	require.Contains(t, content, `RequestBodyRequired: true`)
	require.Contains(t, content, `AuthzMode: "none"`)
	require.Contains(t, content, `Protected: true`)
	require.Contains(t, content, `Manual: true`)
}

func TestEmit_GeneratesPathAndQueryBinding(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{
				Method:      "get",
				Path:        "/groups/{groupId}/members",
				OperationID: "listGroupMembers",
				Parameters: []ir.Parameter{
					{Name: "groupId", In: "path", Required: true, Schema: ir.SchemaRef{Type: "string"}},
					{Name: "max_results", In: "query", Required: false, Schema: ir.SchemaRef{Type: "integer", Format: "int32"}},
				},
				Responses: []ir.Response{{StatusCode: 200, Description: "ok"}},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "ListGroupMembers(w http.ResponseWriter, r *http.Request, groupId string, params GenListGroupMembersParams)")
	require.Contains(t, content, "apigenchi.BindPathParameter(\"groupId\", apigenchi.URLParam(r, \"groupId\"), true, &groupId)")
	require.Contains(t, content, "apigenchi.BindQueryParameter(r.URL.Query(), \"max_results\", false, &params.MaxResults)")
	require.Contains(t, content, "writeAPIGenError(w, http.StatusBadRequest, err.Error())")
	require.Contains(t, content, "dispatcher.ListGroupMembers(w, r, groupId, params)")
	require.Contains(t, content, "type GenListGroupMembersParams struct {")
	require.Contains(t, content, "\tMaxResults *int32")
	require.Contains(t, content, "func apigenErrorMessage(statusCode int, message string) string {")
	require.Contains(t, content, "if statusCode >= http.StatusInternalServerError {")
	require.Contains(t, content, "if statusText := strings.ToLower(http.StatusText(statusCode)); statusText != \"\" {")
	require.Contains(t, content, "func writeAPIGenError(w http.ResponseWriter, statusCode int, message string) {")
	require.Contains(t, content, "_ = json.NewEncoder(w).Encode(Error{Code: apigenchi.SafeIntToInt32(statusCode), Message: apigenErrorMessage(statusCode, message)})")
	require.Contains(t, content, "var request GenListGroupMembersRequest")
	require.Contains(t, content, "\"fmt\"")
	require.Contains(t, content, "response, err := b.handler.ListGroupMembers(r.Context(), request)")
	require.Contains(t, content, "if err := response.VisitListGroupMembersResponse(w); err != nil")
	require.Contains(t, content, "type GenListGroupMembersRequest struct {")
	require.Contains(t, content, "\tGroupId string")
	require.Contains(t, content, "\tParams GenListGroupMembersParams")
	require.Contains(t, content, "type GenListGroupMembersResponse interface {")
	require.Contains(t, content, "\tVisitListGroupMembersResponse(w http.ResponseWriter) error")
	require.Contains(t, content, "type GenListGroupMembers200ResponseHeaders struct {")
	require.Contains(t, content, "type GenListGroupMembers200Response struct {")
	require.Contains(t, content, "\tHeaders GenListGroupMembers200ResponseHeaders")
	require.Contains(t, content, "func (response GenListGroupMembers200Response) VisitListGroupMembersResponse(w http.ResponseWriter) error {")
	require.Contains(t, content, "w.Header().Set(\"X-RateLimit-Limit\", fmt.Sprint(response.Headers.XRateLimitLimit))")
	require.Contains(t, content, "w.WriteHeader(200)")
	require.Contains(t, content, "return nil")
	require.Contains(t, content, "ListGroupMembers(ctx context.Context, request GenListGroupMembersRequest) (GenListGroupMembersResponse, error)")
	require.NotContains(t, content, "type ListGroupMembers200Response = GenListGroupMembers200Response")
}

func TestEmit_GeneratesStrictJSONBodyDecoding(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Schemas: map[string]ir.Schema{
			"CreatePipelineRequest": {Type: "object"},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/pipelines",
				OperationID: "createPipeline",
				RequestBody: &ir.RequestBody{Schema: ir.SchemaRef{Ref: "CreatePipelineRequest"}},
				Responses:   []ir.Response{{StatusCode: 201, Description: "created"}},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "\"io\"")
	require.Contains(t, content, "func decodeAPIGenJSONBody(body io.Reader, dest any, requiredFields ...string) error {")
	require.Contains(t, content, "decoder.DisallowUnknownFields()")
	require.Contains(t, content, "return fmt.Errorf(\"request body must not be empty\")")
	require.Contains(t, content, "return fmt.Errorf(\"request body must contain a single JSON value\")")
	require.Contains(t, content, "decoder := json.NewDecoder(strings.NewReader(string(raw)))")
	require.Contains(t, content, "if err := decodeAPIGenJSONBody(r.Body, &body); err != nil {")
	require.Contains(t, content, "writeAPIGenError(w, http.StatusBadRequest, err.Error())")
	require.Contains(t, content, "writeAPIGenError(w, http.StatusInternalServerError, err.Error())")
	require.Contains(t, content, "Message: apigenErrorMessage(statusCode, message)")
}

func TestEmit_UsesNamedRequestBodySchemas(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Schemas: map[string]ir.Schema{
			"CreateAPIKeyRequest":   {Type: "object"},
			"CreatePipelineRequest": {Type: "object"},
			"MetricQueryRequest":    {Type: "object"},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/api-keys",
				OperationID: "createAPIKey",
				RequestBody: &ir.RequestBody{Schema: ir.SchemaRef{Ref: "CreateAPIKeyRequest"}},
				Responses:   []ir.Response{{StatusCode: 201, Description: "created"}},
			},
			{
				Method:      "post",
				Path:        "/pipelines",
				OperationID: "createPipeline",
				RequestBody: &ir.RequestBody{Schema: ir.SchemaRef{Ref: "GenericRequest"}},
				Responses:   []ir.Response{{StatusCode: 201, Description: "created"}},
			},
			{
				Method:      "post",
				Path:        "/metric-queries:run",
				OperationID: "runMetricQuery",
				RequestBody: &ir.RequestBody{Schema: ir.SchemaRef{Ref: "GenericRequest"}},
				Responses:   []ir.Response{{StatusCode: 201, Description: "created"}},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "type GenCreateAPIKeyJSONBody = GenSchemaCreateAPIKeyRequest")
	require.Contains(t, content, "type GenCreatePipelineJSONBody = GenSchemaCreatePipelineRequest")
	require.Contains(t, content, "type GenRunMetricQueryJSONBody = GenSchemaMetricQueryRequest")
}

func TestEmit_FailsForUnnamedRequestBodySchema(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Schemas: map[string]ir.Schema{
			"GenericRequest": {Type: "object"},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/widgets",
				OperationID: "createWidget",
				RequestBody: &ir.RequestBody{Schema: ir.SchemaRef{Ref: "GenericRequest"}},
				Responses:   []ir.Response{{StatusCode: 201, Description: "created"}},
			},
		},
	}

	_, err := Emit(doc, Options{})
	require.Error(t, err)
	require.ErrorContains(t, err, "request body generation")
	require.ErrorContains(t, err, "createWidget")
}

func TestEmit_ImportsTimeForDateTimeParameters(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{
				Method:      "get",
				Path:        "/audit-logs",
				OperationID: "listAuditLogs",
				Parameters: []ir.Parameter{
					{Name: "from", In: "query", Schema: ir.SchemaRef{Type: "string", Format: "date-time"}},
				},
				Responses: []ir.Response{{StatusCode: 200, Description: "ok"}},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "\"time\"")
	require.Contains(t, content, "type GenListAuditLogsParams struct {")
	require.Contains(t, content, "\tFrom *time.Time")
}

func TestEmit_EmitsCanonicalResponseTypesOnly(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/query",
				OperationID: "executeQuery",
				Responses:   []ir.Response{{StatusCode: 201, Description: "created", Schema: &ir.SchemaRef{Ref: "#/schemas/QueryResult"}}},
			},
			{
				Method:      "post",
				Path:        "/queries",
				OperationID: "submitQuery",
				Responses:   []ir.Response{{StatusCode: 201, Description: "created", Schema: &ir.SchemaRef{Ref: "#/schemas/SubmitQueryResponse"}}},
			},
			{
				Method:      "post",
				Path:        "/queries/{queryId}/cancel",
				OperationID: "cancelQuery",
				Responses:   []ir.Response{{StatusCode: 201, Description: "created", Schema: &ir.SchemaRef{Ref: "#/schemas/CancelQueryResponse"}}},
			},
			{
				Method:      "post",
				Path:        "/security/column-masks/{maskName}/bindings",
				OperationID: "bindColumnMask",
				Responses:   []ir.Response{{StatusCode: 201, Description: "created"}},
			},
			{
				Method:      "get",
				Path:        "/groups",
				OperationID: "listGroups",
				Responses: []ir.Response{
					{StatusCode: 200, Description: "ok", Schema: &ir.SchemaRef{Ref: "#/schemas/PaginatedGroups"}},
					{StatusCode: 403, Description: "forbidden", Schema: &ir.SchemaRef{Ref: "#/schemas/Error"}},
				},
			},
		},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "type GenExecuteQuery201JSONResponse")
	require.Contains(t, content, "type GenSubmitQuery201JSONResponse")
	require.Contains(t, content, "type GenCancelQuery201JSONResponse")
	require.Contains(t, content, "type GenBindColumnMask201Response struct {")
	require.Contains(t, content, "type GenListGroups403JSONResponse struct{ GenForbiddenJSONResponse }")
	require.NotContains(t, content, "type ExecuteQuery200JSONResponse")
	require.NotContains(t, content, "type SubmitQuery202JSONResponse")
	require.NotContains(t, content, "type CancelQuery200JSONResponse")
	require.NotContains(t, content, "type BindColumnMask204Response")
	require.NotContains(t, content, "type BadRequestResponseHeaders =")
	require.NotContains(t, content, "type UnauthorizedJSONResponse =")
}

func TestEmit_UsesIRResponseHeadersForVisitMethods(t *testing.T) {
	t.Helper()

	doc := ir.Document{
		SchemaVersion: "v1",
		Info:          ir.Info{Title: "t", Version: "1"},
		Endpoints: []ir.Endpoint{{
			Method:      "get",
			Path:        "/widgets/{id}",
			OperationID: "getWidget",
			Responses: []ir.Response{{
				StatusCode:  200,
				Description: "ok",
				Headers: []ir.Header{{
					Name:   "X-Trace-Id",
					Schema: ir.SchemaRef{Type: "string"},
				}},
				Schema: &ir.SchemaRef{Type: "string"},
			}},
		}},
	}

	b, err := Emit(doc, Options{})
	require.NoError(t, err)
	content := string(b)

	require.Contains(t, content, "type GenGetWidget200ResponseHeaders struct {")
	require.Contains(t, content, "\tXTraceId string")
	require.Contains(t, content, "w.Header().Set(\"X-Trace-Id\", fmt.Sprint(response.Headers.XTraceId))")
	require.Contains(t, content, "return json.NewEncoder(w).Encode(response.Body)")
}

func TestPathParamTypeName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		param    ir.Parameter
		expected string
	}{
		{
			name:     "default string",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "string"}},
			expected: "string",
		},
		{
			name:     "int32",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "integer", Format: "int32"}},
			expected: "int32",
		},
		{
			name:     "int64",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "integer", Format: "int64"}},
			expected: "int64",
		},
		{
			name:     "integer default",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "integer"}},
			expected: "int",
		},
		{
			name:     "float",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "number", Format: "float"}},
			expected: "float32",
		},
		{
			name:     "double",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "number", Format: "double"}},
			expected: "float64",
		},
		{
			name:     "number default",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "number"}},
			expected: "float64",
		},
		{
			name:     "boolean",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "boolean"}},
			expected: "bool",
		},
		{
			name:     "unknown type fallback",
			param:    ir.Parameter{Schema: ir.SchemaRef{Type: "object"}},
			expected: "string",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			require.Equal(t, tc.expected, pathParamTypeName(tc.param))
		})
	}
}
