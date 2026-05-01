package gen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	apigenchi "github.com/Yacobolo/toolbelt/apigen/runtime/chi"
)

const embeddedOpenAPISpecJSON = `{"components":{"schemas":{"CreateTodoRequest":{"example":{"title":"example"},"properties":{"title":{"example":"example","type":"string"}},"required":["title"],"type":"object"},"Error":{"example":{"code":1,"message":"example"},"properties":{"code":{"example":1,"format":"int32","type":"integer"},"message":{"example":"example","type":"string"}},"required":["code","message"],"type":"object"},"ListTodosResponse":{"example":{"data":[{"id":"example","status":"example","title":"example"}]},"properties":{"data":{"example":[{"id":"example","status":"example","title":"example"}],"items":{"$ref":"#/components/schemas/Todo"},"type":"array"}},"required":["data"],"type":"object"},"Todo":{"example":{"id":"example","status":"example","title":"example"},"properties":{"id":{"example":"example","type":"string"},"status":{"example":"example","type":"string"},"title":{"example":"example","type":"string"}},"required":["id","title","status"],"type":"object"}},"securitySchemes":{"ApiKeyAuth":{"in":"header","name":"X-API-Key","type":"apiKey"},"BearerAuth":{"scheme":"Bearer","type":"http"}}},"info":{"description":"Small in-memory todo API authored in CUE to showcase APIGen generation and strict server wiring.","title":"APIGen Todo Example","version":"0.1.0"},"openapi":"3.0.0","paths":{"/todos":{"get":{"operationId":"listTodos","parameters":[{"description":"Optional status filter.","example":"example","explode":false,"in":"query","name":"status","required":false,"schema":{"type":"string"}}],"responses":{"200":{"content":{"application/json":{"example":{"data":[{"id":"example","status":"example","title":"example"}]},"schema":{"$ref":"#/components/schemas/ListTodosResponse"}}},"description":"The request has succeeded."},"400":{"content":{"application/json":{"example":{"code":1,"message":"example"},"schema":{"$ref":"#/components/schemas/Error"}}},"description":"The request is invalid."}},"summary":"List todos","tags":["Todos"]},"post":{"operationId":"createTodo","parameters":[],"requestBody":{"content":{"application/json":{"example":{"title":"example"},"schema":{"$ref":"#/components/schemas/CreateTodoRequest"}}},"required":true},"responses":{"201":{"content":{"application/json":{"example":{"id":"example","status":"example","title":"example"},"schema":{"$ref":"#/components/schemas/Todo"}}},"description":"A todo has been created."},"400":{"content":{"application/json":{"example":{"code":1,"message":"example"},"schema":{"$ref":"#/components/schemas/Error"}}},"description":"The request is invalid."}},"summary":"Create todo","tags":["Todos"]}},"/todos/{todo_id}":{"delete":{"operationId":"deleteTodo","parameters":[{"example":"example","in":"path","name":"todo_id","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"The todo has been deleted."},"404":{"content":{"application/json":{"example":{"code":1,"message":"example"},"schema":{"$ref":"#/components/schemas/Error"}}},"description":"The todo was not found."}},"summary":"Delete todo","tags":["Todos"]},"get":{"operationId":"getTodo","parameters":[{"example":"example","in":"path","name":"todo_id","required":true,"schema":{"type":"string"}}],"responses":{"200":{"content":{"application/json":{"example":{"id":"example","status":"example","title":"example"},"schema":{"$ref":"#/components/schemas/Todo"}}},"description":"The request has succeeded."},"404":{"content":{"application/json":{"example":{"code":1,"message":"example"},"schema":{"$ref":"#/components/schemas/Error"}}},"description":"The todo was not found."}},"summary":"Get todo","tags":["Todos"]}},"/todos/{todo_id}/complete":{"post":{"operationId":"completeTodo","parameters":[{"example":"example","in":"path","name":"todo_id","required":true,"schema":{"type":"string"}}],"responses":{"200":{"content":{"application/json":{"example":{"id":"example","status":"example","title":"example"},"schema":{"$ref":"#/components/schemas/Todo"}}},"description":"The todo has been completed."},"404":{"content":{"application/json":{"example":{"code":1,"message":"example"},"schema":{"$ref":"#/components/schemas/Error"}}},"description":"The todo was not found."}},"summary":"Complete todo","tags":["Todos"]}}},"servers":[{"description":"Example development server","url":"http://127.0.0.1:8081/","variables":{}}],"tags":[{"description":"Todo lifecycle endpoints for the APIGen example.","name":"Todos"}]}`

// GetEmbeddedOpenAPISpec returns the canonical OpenAPI document as generic JSON map.
func GetEmbeddedOpenAPISpec() (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(embeddedOpenAPISpecJSON), &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// GenOperationContract captures APIGen-owned contract metadata for one operation.
type GenOperationContract struct {
	OperationID           string
	Method                string
	Path                  string
	Tags                  []string
	DocumentedStatusCodes []int
	RequestBodyRequired   bool
	AuthzMode             string
	Protected             bool
	Manual                bool
}

var genOperationContracts = map[string]GenOperationContract{
	"listTodos":    {OperationID: "listTodos", Method: "GET", Path: "/todos", Tags: []string{"Todos"}, DocumentedStatusCodes: []int{200, 400, 401, 403, 404, 409, 429, 500, 502}, RequestBodyRequired: false, AuthzMode: "", Protected: false, Manual: false},
	"createTodo":   {OperationID: "createTodo", Method: "POST", Path: "/todos", Tags: []string{"Todos"}, DocumentedStatusCodes: []int{201, 400, 401, 403, 404, 409, 429, 500, 502}, RequestBodyRequired: true, AuthzMode: "", Protected: false, Manual: false},
	"deleteTodo":   {OperationID: "deleteTodo", Method: "DELETE", Path: "/todos/{todo_id}", Tags: []string{"Todos"}, DocumentedStatusCodes: []int{204, 400, 401, 403, 404, 409, 429, 500, 502}, RequestBodyRequired: false, AuthzMode: "", Protected: false, Manual: false},
	"getTodo":      {OperationID: "getTodo", Method: "GET", Path: "/todos/{todo_id}", Tags: []string{"Todos"}, DocumentedStatusCodes: []int{200, 400, 401, 403, 404, 409, 429, 500, 502}, RequestBodyRequired: false, AuthzMode: "", Protected: false, Manual: false},
	"completeTodo": {OperationID: "completeTodo", Method: "POST", Path: "/todos/{todo_id}/complete", Tags: []string{"Todos"}, DocumentedStatusCodes: []int{200, 400, 401, 403, 404, 409, 429, 500, 502}, RequestBodyRequired: false, AuthzMode: "", Protected: false, Manual: false},
}

// GetAPIGenOperationContracts returns a defensive copy of the generated contract registry.
func GetAPIGenOperationContracts() map[string]GenOperationContract {
	out := make(map[string]GenOperationContract, len(genOperationContracts))
	for operationID, contract := range genOperationContracts {
		out[operationID] = cloneAPIGenOperationContract(contract)
	}
	return out
}

// GetAPIGenOperationContract returns generated contract metadata for a single operation.
func GetAPIGenOperationContract(operationID string) (GenOperationContract, bool) {
	contract, ok := genOperationContracts[operationID]
	if !ok {
		return GenOperationContract{}, false
	}
	return cloneAPIGenOperationContract(contract), true
}

// APIGenOperationAllowsStatus reports whether a status code is documented for an operation.
//
//nolint:revive // exported generated helper name matches the APIGen contract registry namespace.
func APIGenOperationAllowsStatus(operationID string, statusCode int) bool {
	contract, ok := genOperationContracts[operationID]
	if !ok {
		return false
	}
	for _, documented := range contract.DocumentedStatusCodes {
		if documented == statusCode {
			return true
		}
	}
	return false
}

func cloneAPIGenOperationContract(contract GenOperationContract) GenOperationContract {
	contract.Tags = append([]string(nil), contract.Tags...)
	contract.DocumentedStatusCodes = append([]int(nil), contract.DocumentedStatusCodes...)
	return contract
}

// GenServerInterface dispatches generated operations.
type GenServerInterface interface {
	HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request)
}

// RegisterAPIGenRoutes mounts generated routes on the supported Chi runtime boundary.
func RegisterAPIGenRoutes(router apigenchi.Router, server GenServerInterface) {
	apigenchi.RegisterRoutes(router, []apigenchi.Route{
		{Method: "GET", Path: "/todos", OperationID: "listTodos"},
		{Method: "POST", Path: "/todos", OperationID: "createTodo"},
		{Method: "DELETE", Path: "/todos/{todo_id}", OperationID: "deleteTodo"},
		{Method: "GET", Path: "/todos/{todo_id}", OperationID: "getTodo"},
		{Method: "POST", Path: "/todos/{todo_id}/complete", OperationID: "completeTodo"},
	}, server.HandleAPIGen)
}

// RegisterAPIGenStrictRoutes mounts generated routes backed by strict handlers.
func RegisterAPIGenStrictRoutes(router apigenchi.Router, handler GenStrictServerInterface) {
	RegisterAPIGenRoutes(router, genStrictAdapter{handler: handler})
}

// GenOperationDispatcher is the dispatch target for generated operations.
type GenOperationDispatcher interface {
	ListTodos(w http.ResponseWriter, r *http.Request, params GenListTodosParams)
	CreateTodo(w http.ResponseWriter, r *http.Request)
	DeleteTodo(w http.ResponseWriter, r *http.Request, todoId string)
	GetTodo(w http.ResponseWriter, r *http.Request, todoId string)
	CompleteTodo(w http.ResponseWriter, r *http.Request, todoId string)
}

// DispatchAPIGenOperation dispatches operation IDs to generated wrapper methods.
func DispatchAPIGenOperation(operationID string, dispatcher GenOperationDispatcher, w http.ResponseWriter, r *http.Request) bool {
	switch operationID {
	case "listTodos":
		var err error
		var params GenListTodosParams
		err = apigenchi.BindQueryParameter(r.URL.Query(), "status", false, &params.Status)
		if err != nil {
			writeAPIGenError(w, http.StatusBadRequest, err.Error())
			return true
		}
		dispatcher.ListTodos(w, r, params)
		return true
	case "createTodo":
		dispatcher.CreateTodo(w, r)
		return true
	case "deleteTodo":
		var err error
		var todoId string
		err = apigenchi.BindPathParameter("todo_id", apigenchi.URLParam(r, "todo_id"), true, &todoId)
		if err != nil {
			writeAPIGenError(w, http.StatusBadRequest, err.Error())
			return true
		}
		dispatcher.DeleteTodo(w, r, todoId)
		return true
	case "getTodo":
		var err error
		var todoId string
		err = apigenchi.BindPathParameter("todo_id", apigenchi.URLParam(r, "todo_id"), true, &todoId)
		if err != nil {
			writeAPIGenError(w, http.StatusBadRequest, err.Error())
			return true
		}
		dispatcher.GetTodo(w, r, todoId)
		return true
	case "completeTodo":
		var err error
		var todoId string
		err = apigenchi.BindPathParameter("todo_id", apigenchi.URLParam(r, "todo_id"), true, &todoId)
		if err != nil {
			writeAPIGenError(w, http.StatusBadRequest, err.Error())
			return true
		}
		dispatcher.CompleteTodo(w, r, todoId)
		return true
	default:
		return false
	}
}

func apigenErrorMessage(statusCode int, message string) string {
	if statusCode >= http.StatusInternalServerError {
		if statusText := strings.ToLower(http.StatusText(statusCode)); statusText != "" {
			return statusText
		}
	}
	return message
}

func writeAPIGenError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(Error{Code: apigenchi.SafeIntToInt32(statusCode), Message: apigenErrorMessage(statusCode, message)})
}

func decodeAPIGenJSONBody(body io.Reader, dest any, requiredFields ...string) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("request body must not be empty")
	}
	if len(requiredFields) > 0 {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err == nil {
			for _, field := range requiredFields {
				if _, ok := envelope[field]; !ok {
					return fmt.Errorf("%s is required", field)
				}
			}
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

// GenBadRequestResponseHeaders represents the APIGen shared response headers for BadRequest JSON errors.
type GenBadRequestResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenBadRequestJSONResponse represents the APIGen shared JSON error body for BadRequest responses.
type GenBadRequestJSONResponse struct {
	Body Error

	Headers GenBadRequestResponseHeaders
}

// GenConflictResponseHeaders represents the APIGen shared response headers for Conflict JSON errors.
type GenConflictResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenConflictJSONResponse represents the APIGen shared JSON error body for Conflict responses.
type GenConflictJSONResponse struct {
	Body Error

	Headers GenConflictResponseHeaders
}

// GenForbiddenResponseHeaders represents the APIGen shared response headers for Forbidden JSON errors.
type GenForbiddenResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenForbiddenJSONResponse represents the APIGen shared JSON error body for Forbidden responses.
type GenForbiddenJSONResponse struct {
	Body Error

	Headers GenForbiddenResponseHeaders
}

// GenInternalErrorResponseHeaders represents the APIGen shared response headers for InternalError JSON errors.
type GenInternalErrorResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenInternalErrorJSONResponse represents the APIGen shared JSON error body for InternalError responses.
type GenInternalErrorJSONResponse struct {
	Body Error

	Headers GenInternalErrorResponseHeaders
}

// GenNotFoundResponseHeaders represents the APIGen shared response headers for NotFound JSON errors.
type GenNotFoundResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenNotFoundJSONResponse represents the APIGen shared JSON error body for NotFound responses.
type GenNotFoundJSONResponse struct {
	Body Error

	Headers GenNotFoundResponseHeaders
}

// GenRateLimitExceededResponseHeaders represents the APIGen shared response headers for RateLimitExceeded JSON errors.
type GenRateLimitExceededResponseHeaders struct {
	RetryAfter          int32
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenRateLimitExceededJSONResponse represents the APIGen shared JSON error body for RateLimitExceeded responses.
type GenRateLimitExceededJSONResponse struct {
	Body Error

	Headers GenRateLimitExceededResponseHeaders
}

// GenUnauthorizedResponseHeaders represents the APIGen shared response headers for Unauthorized JSON errors.
type GenUnauthorizedResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenUnauthorizedJSONResponse represents the APIGen shared JSON error body for Unauthorized responses.
type GenUnauthorizedJSONResponse struct {
	Body Error

	Headers GenUnauthorizedResponseHeaders
}

// GenListTodosParams represents the APIGen strict query parameter contract for ListTodos.
type GenListTodosParams struct {
	Status *string
}

// GenListTodosRequest represents the APIGen strict request contract for ListTodos.
type GenListTodosRequest struct {
	Params GenListTodosParams
}

// GenListTodosResponse represents the APIGen strict response contract for ListTodos.
type GenListTodosResponse interface {
	VisitListTodosResponse(w http.ResponseWriter) error
}

// GenListTodos200ResponseHeaders represents the APIGen-owned response headers for generated concrete responses.
type GenListTodos200ResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenListTodos200JSONResponse is the APIGen concrete JSON response for ListTodos 200.
type GenListTodos200JSONResponse struct {
	Body    GenSchemaListTodosResponse
	Headers GenListTodos200ResponseHeaders
}

// VisitListTodosResponse writes ListTodos 200 responses to the client.
func (response GenListTodos200JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos400ResponseHeaders aliases the APIGen shared response headers for ListTodos 400 errors.
type GenListTodos400ResponseHeaders = GenBadRequestResponseHeaders

// GenListTodos400JSONResponse is the APIGen concrete JSON response for ListTodos 400.
type GenListTodos400JSONResponse struct{ GenBadRequestJSONResponse }

// VisitListTodosResponse writes ListTodos 400 responses to the client.
func (response GenListTodos400JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos401ResponseHeaders aliases the APIGen shared response headers for ListTodos 401 errors.
type GenListTodos401ResponseHeaders = GenUnauthorizedResponseHeaders

// GenListTodos401JSONResponse is the APIGen shared JSON response for ListTodos 401.
type GenListTodos401JSONResponse struct{ GenUnauthorizedJSONResponse }

// VisitListTodosResponse writes ListTodos 401 responses to the client.
func (response GenListTodos401JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos403ResponseHeaders aliases the APIGen shared response headers for ListTodos 403 errors.
type GenListTodos403ResponseHeaders = GenForbiddenResponseHeaders

// GenListTodos403JSONResponse is the APIGen shared JSON response for ListTodos 403.
type GenListTodos403JSONResponse struct{ GenForbiddenJSONResponse }

// VisitListTodosResponse writes ListTodos 403 responses to the client.
func (response GenListTodos403JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(403)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos404ResponseHeaders aliases the APIGen shared response headers for ListTodos 404 errors.
type GenListTodos404ResponseHeaders = GenNotFoundResponseHeaders

// GenListTodos404JSONResponse is the APIGen shared JSON response for ListTodos 404.
type GenListTodos404JSONResponse struct{ GenNotFoundJSONResponse }

// VisitListTodosResponse writes ListTodos 404 responses to the client.
func (response GenListTodos404JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(404)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos409ResponseHeaders aliases the APIGen shared response headers for ListTodos 409 errors.
type GenListTodos409ResponseHeaders = GenConflictResponseHeaders

// GenListTodos409JSONResponse is the APIGen shared JSON response for ListTodos 409.
type GenListTodos409JSONResponse struct{ GenConflictJSONResponse }

// VisitListTodosResponse writes ListTodos 409 responses to the client.
func (response GenListTodos409JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(409)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos429ResponseHeaders aliases the APIGen shared response headers for ListTodos 429 errors.
type GenListTodos429ResponseHeaders = GenRateLimitExceededResponseHeaders

// GenListTodos429JSONResponse is the APIGen shared JSON response for ListTodos 429.
type GenListTodos429JSONResponse struct {
	GenRateLimitExceededJSONResponse
}

// VisitListTodosResponse writes ListTodos 429 responses to the client.
func (response GenListTodos429JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", fmt.Sprint(response.Headers.RetryAfter))
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(429)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos500ResponseHeaders aliases the APIGen shared response headers for ListTodos 500 errors.
type GenListTodos500ResponseHeaders = GenInternalErrorResponseHeaders

// GenListTodos500JSONResponse is the APIGen shared JSON response for ListTodos 500.
type GenListTodos500JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitListTodosResponse writes ListTodos 500 responses to the client.
func (response GenListTodos500JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenListTodos502ResponseHeaders aliases the APIGen shared response headers for ListTodos 502 errors.
type GenListTodos502ResponseHeaders = GenInternalErrorResponseHeaders

// GenListTodos502JSONResponse is the APIGen shared JSON response for ListTodos 502.
type GenListTodos502JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitListTodosResponse writes ListTodos 502 responses to the client.
func (response GenListTodos502JSONResponse) VisitListTodosResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(502)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodoRequest represents the APIGen strict request contract for CreateTodo.
type GenCreateTodoRequest struct {
	Body *GenCreateTodoJSONBody
}

// GenCreateTodoResponse represents the APIGen strict response contract for CreateTodo.
type GenCreateTodoResponse interface {
	VisitCreateTodoResponse(w http.ResponseWriter) error
}

// GenCreateTodo201ResponseHeaders represents the APIGen-owned response headers for generated concrete responses.
type GenCreateTodo201ResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenCreateTodo201JSONResponse is the APIGen concrete JSON response for CreateTodo 201.
type GenCreateTodo201JSONResponse struct {
	Body    GenSchemaTodo
	Headers GenCreateTodo201ResponseHeaders
}

// VisitCreateTodoResponse writes CreateTodo 201 responses to the client.
func (response GenCreateTodo201JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo400ResponseHeaders aliases the APIGen shared response headers for CreateTodo 400 errors.
type GenCreateTodo400ResponseHeaders = GenBadRequestResponseHeaders

// GenCreateTodo400JSONResponse is the APIGen concrete JSON response for CreateTodo 400.
type GenCreateTodo400JSONResponse struct{ GenBadRequestJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 400 responses to the client.
func (response GenCreateTodo400JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo401ResponseHeaders aliases the APIGen shared response headers for CreateTodo 401 errors.
type GenCreateTodo401ResponseHeaders = GenUnauthorizedResponseHeaders

// GenCreateTodo401JSONResponse is the APIGen shared JSON response for CreateTodo 401.
type GenCreateTodo401JSONResponse struct{ GenUnauthorizedJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 401 responses to the client.
func (response GenCreateTodo401JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo403ResponseHeaders aliases the APIGen shared response headers for CreateTodo 403 errors.
type GenCreateTodo403ResponseHeaders = GenForbiddenResponseHeaders

// GenCreateTodo403JSONResponse is the APIGen shared JSON response for CreateTodo 403.
type GenCreateTodo403JSONResponse struct{ GenForbiddenJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 403 responses to the client.
func (response GenCreateTodo403JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(403)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo404ResponseHeaders aliases the APIGen shared response headers for CreateTodo 404 errors.
type GenCreateTodo404ResponseHeaders = GenNotFoundResponseHeaders

// GenCreateTodo404JSONResponse is the APIGen shared JSON response for CreateTodo 404.
type GenCreateTodo404JSONResponse struct{ GenNotFoundJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 404 responses to the client.
func (response GenCreateTodo404JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(404)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo409ResponseHeaders aliases the APIGen shared response headers for CreateTodo 409 errors.
type GenCreateTodo409ResponseHeaders = GenConflictResponseHeaders

// GenCreateTodo409JSONResponse is the APIGen shared JSON response for CreateTodo 409.
type GenCreateTodo409JSONResponse struct{ GenConflictJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 409 responses to the client.
func (response GenCreateTodo409JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(409)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo429ResponseHeaders aliases the APIGen shared response headers for CreateTodo 429 errors.
type GenCreateTodo429ResponseHeaders = GenRateLimitExceededResponseHeaders

// GenCreateTodo429JSONResponse is the APIGen shared JSON response for CreateTodo 429.
type GenCreateTodo429JSONResponse struct {
	GenRateLimitExceededJSONResponse
}

// VisitCreateTodoResponse writes CreateTodo 429 responses to the client.
func (response GenCreateTodo429JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", fmt.Sprint(response.Headers.RetryAfter))
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(429)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo500ResponseHeaders aliases the APIGen shared response headers for CreateTodo 500 errors.
type GenCreateTodo500ResponseHeaders = GenInternalErrorResponseHeaders

// GenCreateTodo500JSONResponse is the APIGen shared JSON response for CreateTodo 500.
type GenCreateTodo500JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 500 responses to the client.
func (response GenCreateTodo500JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodo502ResponseHeaders aliases the APIGen shared response headers for CreateTodo 502 errors.
type GenCreateTodo502ResponseHeaders = GenInternalErrorResponseHeaders

// GenCreateTodo502JSONResponse is the APIGen shared JSON response for CreateTodo 502.
type GenCreateTodo502JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitCreateTodoResponse writes CreateTodo 502 responses to the client.
func (response GenCreateTodo502JSONResponse) VisitCreateTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(502)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCreateTodoJSONBody aliases the APIGen strict JSON request body schema for CreateTodo.
type GenCreateTodoJSONBody = GenSchemaCreateTodoRequest

// GenDeleteTodoRequest represents the APIGen strict request contract for DeleteTodo.
type GenDeleteTodoRequest struct {
	TodoId string
}

// GenDeleteTodoResponse represents the APIGen strict response contract for DeleteTodo.
type GenDeleteTodoResponse interface {
	VisitDeleteTodoResponse(w http.ResponseWriter) error
}

// GenDeleteTodo204ResponseHeaders represents the APIGen-owned response headers for generated concrete responses.
type GenDeleteTodo204ResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenDeleteTodo204Response is the APIGen concrete response for DeleteTodo 204.
type GenDeleteTodo204Response struct {
	Headers GenDeleteTodo204ResponseHeaders
}

// VisitDeleteTodoResponse writes DeleteTodo 204 responses to the client.
func (response GenDeleteTodo204Response) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.WriteHeader(204)
	return nil
}

// GenDeleteTodo404ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 404 errors.
type GenDeleteTodo404ResponseHeaders = GenNotFoundResponseHeaders

// GenDeleteTodo404JSONResponse is the APIGen concrete JSON response for DeleteTodo 404.
type GenDeleteTodo404JSONResponse struct{ GenNotFoundJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 404 responses to the client.
func (response GenDeleteTodo404JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(404)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo400ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 400 errors.
type GenDeleteTodo400ResponseHeaders = GenBadRequestResponseHeaders

// GenDeleteTodo400JSONResponse is the APIGen shared JSON response for DeleteTodo 400.
type GenDeleteTodo400JSONResponse struct{ GenBadRequestJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 400 responses to the client.
func (response GenDeleteTodo400JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo401ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 401 errors.
type GenDeleteTodo401ResponseHeaders = GenUnauthorizedResponseHeaders

// GenDeleteTodo401JSONResponse is the APIGen shared JSON response for DeleteTodo 401.
type GenDeleteTodo401JSONResponse struct{ GenUnauthorizedJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 401 responses to the client.
func (response GenDeleteTodo401JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo403ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 403 errors.
type GenDeleteTodo403ResponseHeaders = GenForbiddenResponseHeaders

// GenDeleteTodo403JSONResponse is the APIGen shared JSON response for DeleteTodo 403.
type GenDeleteTodo403JSONResponse struct{ GenForbiddenJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 403 responses to the client.
func (response GenDeleteTodo403JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(403)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo409ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 409 errors.
type GenDeleteTodo409ResponseHeaders = GenConflictResponseHeaders

// GenDeleteTodo409JSONResponse is the APIGen shared JSON response for DeleteTodo 409.
type GenDeleteTodo409JSONResponse struct{ GenConflictJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 409 responses to the client.
func (response GenDeleteTodo409JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(409)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo429ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 429 errors.
type GenDeleteTodo429ResponseHeaders = GenRateLimitExceededResponseHeaders

// GenDeleteTodo429JSONResponse is the APIGen shared JSON response for DeleteTodo 429.
type GenDeleteTodo429JSONResponse struct {
	GenRateLimitExceededJSONResponse
}

// VisitDeleteTodoResponse writes DeleteTodo 429 responses to the client.
func (response GenDeleteTodo429JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", fmt.Sprint(response.Headers.RetryAfter))
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(429)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo500ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 500 errors.
type GenDeleteTodo500ResponseHeaders = GenInternalErrorResponseHeaders

// GenDeleteTodo500JSONResponse is the APIGen shared JSON response for DeleteTodo 500.
type GenDeleteTodo500JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 500 responses to the client.
func (response GenDeleteTodo500JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenDeleteTodo502ResponseHeaders aliases the APIGen shared response headers for DeleteTodo 502 errors.
type GenDeleteTodo502ResponseHeaders = GenInternalErrorResponseHeaders

// GenDeleteTodo502JSONResponse is the APIGen shared JSON response for DeleteTodo 502.
type GenDeleteTodo502JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitDeleteTodoResponse writes DeleteTodo 502 responses to the client.
func (response GenDeleteTodo502JSONResponse) VisitDeleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(502)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodoRequest represents the APIGen strict request contract for GetTodo.
type GenGetTodoRequest struct {
	TodoId string
}

// GenGetTodoResponse represents the APIGen strict response contract for GetTodo.
type GenGetTodoResponse interface {
	VisitGetTodoResponse(w http.ResponseWriter) error
}

// GenGetTodo200ResponseHeaders represents the APIGen-owned response headers for generated concrete responses.
type GenGetTodo200ResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenGetTodo200JSONResponse is the APIGen concrete JSON response for GetTodo 200.
type GenGetTodo200JSONResponse struct {
	Body    GenSchemaTodo
	Headers GenGetTodo200ResponseHeaders
}

// VisitGetTodoResponse writes GetTodo 200 responses to the client.
func (response GenGetTodo200JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo404ResponseHeaders aliases the APIGen shared response headers for GetTodo 404 errors.
type GenGetTodo404ResponseHeaders = GenNotFoundResponseHeaders

// GenGetTodo404JSONResponse is the APIGen concrete JSON response for GetTodo 404.
type GenGetTodo404JSONResponse struct{ GenNotFoundJSONResponse }

// VisitGetTodoResponse writes GetTodo 404 responses to the client.
func (response GenGetTodo404JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(404)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo400ResponseHeaders aliases the APIGen shared response headers for GetTodo 400 errors.
type GenGetTodo400ResponseHeaders = GenBadRequestResponseHeaders

// GenGetTodo400JSONResponse is the APIGen shared JSON response for GetTodo 400.
type GenGetTodo400JSONResponse struct{ GenBadRequestJSONResponse }

// VisitGetTodoResponse writes GetTodo 400 responses to the client.
func (response GenGetTodo400JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo401ResponseHeaders aliases the APIGen shared response headers for GetTodo 401 errors.
type GenGetTodo401ResponseHeaders = GenUnauthorizedResponseHeaders

// GenGetTodo401JSONResponse is the APIGen shared JSON response for GetTodo 401.
type GenGetTodo401JSONResponse struct{ GenUnauthorizedJSONResponse }

// VisitGetTodoResponse writes GetTodo 401 responses to the client.
func (response GenGetTodo401JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo403ResponseHeaders aliases the APIGen shared response headers for GetTodo 403 errors.
type GenGetTodo403ResponseHeaders = GenForbiddenResponseHeaders

// GenGetTodo403JSONResponse is the APIGen shared JSON response for GetTodo 403.
type GenGetTodo403JSONResponse struct{ GenForbiddenJSONResponse }

// VisitGetTodoResponse writes GetTodo 403 responses to the client.
func (response GenGetTodo403JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(403)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo409ResponseHeaders aliases the APIGen shared response headers for GetTodo 409 errors.
type GenGetTodo409ResponseHeaders = GenConflictResponseHeaders

// GenGetTodo409JSONResponse is the APIGen shared JSON response for GetTodo 409.
type GenGetTodo409JSONResponse struct{ GenConflictJSONResponse }

// VisitGetTodoResponse writes GetTodo 409 responses to the client.
func (response GenGetTodo409JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(409)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo429ResponseHeaders aliases the APIGen shared response headers for GetTodo 429 errors.
type GenGetTodo429ResponseHeaders = GenRateLimitExceededResponseHeaders

// GenGetTodo429JSONResponse is the APIGen shared JSON response for GetTodo 429.
type GenGetTodo429JSONResponse struct {
	GenRateLimitExceededJSONResponse
}

// VisitGetTodoResponse writes GetTodo 429 responses to the client.
func (response GenGetTodo429JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", fmt.Sprint(response.Headers.RetryAfter))
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(429)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo500ResponseHeaders aliases the APIGen shared response headers for GetTodo 500 errors.
type GenGetTodo500ResponseHeaders = GenInternalErrorResponseHeaders

// GenGetTodo500JSONResponse is the APIGen shared JSON response for GetTodo 500.
type GenGetTodo500JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitGetTodoResponse writes GetTodo 500 responses to the client.
func (response GenGetTodo500JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenGetTodo502ResponseHeaders aliases the APIGen shared response headers for GetTodo 502 errors.
type GenGetTodo502ResponseHeaders = GenInternalErrorResponseHeaders

// GenGetTodo502JSONResponse is the APIGen shared JSON response for GetTodo 502.
type GenGetTodo502JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitGetTodoResponse writes GetTodo 502 responses to the client.
func (response GenGetTodo502JSONResponse) VisitGetTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(502)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodoRequest represents the APIGen strict request contract for CompleteTodo.
type GenCompleteTodoRequest struct {
	TodoId string
}

// GenCompleteTodoResponse represents the APIGen strict response contract for CompleteTodo.
type GenCompleteTodoResponse interface {
	VisitCompleteTodoResponse(w http.ResponseWriter) error
}

// GenCompleteTodo200ResponseHeaders represents the APIGen-owned response headers for generated concrete responses.
type GenCompleteTodo200ResponseHeaders struct {
	XRateLimitLimit     int32
	XRateLimitRemaining int32
	XRateLimitReset     int64
}

// GenCompleteTodo200JSONResponse is the APIGen concrete JSON response for CompleteTodo 200.
type GenCompleteTodo200JSONResponse struct {
	Body    GenSchemaTodo
	Headers GenCompleteTodo200ResponseHeaders
}

// VisitCompleteTodoResponse writes CompleteTodo 200 responses to the client.
func (response GenCompleteTodo200JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo404ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 404 errors.
type GenCompleteTodo404ResponseHeaders = GenNotFoundResponseHeaders

// GenCompleteTodo404JSONResponse is the APIGen concrete JSON response for CompleteTodo 404.
type GenCompleteTodo404JSONResponse struct{ GenNotFoundJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 404 responses to the client.
func (response GenCompleteTodo404JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(404)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo400ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 400 errors.
type GenCompleteTodo400ResponseHeaders = GenBadRequestResponseHeaders

// GenCompleteTodo400JSONResponse is the APIGen shared JSON response for CompleteTodo 400.
type GenCompleteTodo400JSONResponse struct{ GenBadRequestJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 400 responses to the client.
func (response GenCompleteTodo400JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo401ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 401 errors.
type GenCompleteTodo401ResponseHeaders = GenUnauthorizedResponseHeaders

// GenCompleteTodo401JSONResponse is the APIGen shared JSON response for CompleteTodo 401.
type GenCompleteTodo401JSONResponse struct{ GenUnauthorizedJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 401 responses to the client.
func (response GenCompleteTodo401JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo403ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 403 errors.
type GenCompleteTodo403ResponseHeaders = GenForbiddenResponseHeaders

// GenCompleteTodo403JSONResponse is the APIGen shared JSON response for CompleteTodo 403.
type GenCompleteTodo403JSONResponse struct{ GenForbiddenJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 403 responses to the client.
func (response GenCompleteTodo403JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(403)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo409ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 409 errors.
type GenCompleteTodo409ResponseHeaders = GenConflictResponseHeaders

// GenCompleteTodo409JSONResponse is the APIGen shared JSON response for CompleteTodo 409.
type GenCompleteTodo409JSONResponse struct{ GenConflictJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 409 responses to the client.
func (response GenCompleteTodo409JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(409)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo429ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 429 errors.
type GenCompleteTodo429ResponseHeaders = GenRateLimitExceededResponseHeaders

// GenCompleteTodo429JSONResponse is the APIGen shared JSON response for CompleteTodo 429.
type GenCompleteTodo429JSONResponse struct {
	GenRateLimitExceededJSONResponse
}

// VisitCompleteTodoResponse writes CompleteTodo 429 responses to the client.
func (response GenCompleteTodo429JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("Retry-After", fmt.Sprint(response.Headers.RetryAfter))
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(429)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo500ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 500 errors.
type GenCompleteTodo500ResponseHeaders = GenInternalErrorResponseHeaders

// GenCompleteTodo500JSONResponse is the APIGen shared JSON response for CompleteTodo 500.
type GenCompleteTodo500JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 500 responses to the client.
func (response GenCompleteTodo500JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenCompleteTodo502ResponseHeaders aliases the APIGen shared response headers for CompleteTodo 502 errors.
type GenCompleteTodo502ResponseHeaders = GenInternalErrorResponseHeaders

// GenCompleteTodo502JSONResponse is the APIGen shared JSON response for CompleteTodo 502.
type GenCompleteTodo502JSONResponse struct{ GenInternalErrorJSONResponse }

// VisitCompleteTodoResponse writes CompleteTodo 502 responses to the client.
func (response GenCompleteTodo502JSONResponse) VisitCompleteTodoResponse(w http.ResponseWriter) error {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprint(response.Headers.XRateLimitLimit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(response.Headers.XRateLimitRemaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprint(response.Headers.XRateLimitReset))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(502)
	return json.NewEncoder(w).Encode(response.Body)
}

// GenStrictServerInterface represents strict handlers for APIGen transport dispatch.
type GenStrictServerInterface interface {
	ListTodos(ctx context.Context, request GenListTodosRequest) (GenListTodosResponse, error)
	CreateTodo(ctx context.Context, request GenCreateTodoRequest) (GenCreateTodoResponse, error)
	DeleteTodo(ctx context.Context, request GenDeleteTodoRequest) (GenDeleteTodoResponse, error)
	GetTodo(ctx context.Context, request GenGetTodoRequest) (GenGetTodoResponse, error)
	CompleteTodo(ctx context.Context, request GenCompleteTodoRequest) (GenCompleteTodoResponse, error)
}

type genStrictAdapter struct {
	handler GenStrictServerInterface
}

func (a genStrictAdapter) HandleAPIGen(operationID string, w http.ResponseWriter, r *http.Request) {
	if ok := DispatchAPIGenStrictOperation(operationID, a.handler, w, r); !ok {
		http.NotFound(w, r)
	}
}

type genStrictBridge struct {
	handler GenStrictServerInterface
}

func (b genStrictBridge) ListTodos(w http.ResponseWriter, r *http.Request, params GenListTodosParams) {
	var request GenListTodosRequest
	request.Params = params
	response, err := b.handler.ListTodos(r.Context(), request)
	if err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := response.VisitListTodosResponse(w); err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
	}
}

func (b genStrictBridge) CreateTodo(w http.ResponseWriter, r *http.Request) {
	var request GenCreateTodoRequest
	var body GenCreateTodoJSONBody
	if err := decodeAPIGenJSONBody(r.Body, &body, []string{"title"}...); err != nil {
		writeAPIGenError(w, http.StatusBadRequest, err.Error())
		return
	}
	request.Body = &body
	response, err := b.handler.CreateTodo(r.Context(), request)
	if err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := response.VisitCreateTodoResponse(w); err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
	}
}

func (b genStrictBridge) DeleteTodo(w http.ResponseWriter, r *http.Request, todoId string) {
	var request GenDeleteTodoRequest
	request.TodoId = todoId
	response, err := b.handler.DeleteTodo(r.Context(), request)
	if err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := response.VisitDeleteTodoResponse(w); err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
	}
}

func (b genStrictBridge) GetTodo(w http.ResponseWriter, r *http.Request, todoId string) {
	var request GenGetTodoRequest
	request.TodoId = todoId
	response, err := b.handler.GetTodo(r.Context(), request)
	if err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := response.VisitGetTodoResponse(w); err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
	}
}

func (b genStrictBridge) CompleteTodo(w http.ResponseWriter, r *http.Request, todoId string) {
	var request GenCompleteTodoRequest
	request.TodoId = todoId
	response, err := b.handler.CompleteTodo(r.Context(), request)
	if err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := response.VisitCompleteTodoResponse(w); err != nil {
		writeAPIGenError(w, http.StatusInternalServerError, err.Error())
	}
}

// DispatchAPIGenStrictOperation dispatches to strict handlers without oapi strict wrappers.
func DispatchAPIGenStrictOperation(operationID string, handler GenStrictServerInterface, w http.ResponseWriter, r *http.Request) bool {
	return DispatchAPIGenOperation(operationID, genStrictBridge{handler: handler}, w, r)
}
