package ir

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CurrentSchemaVersion is the supported JSON IR schema version.
const CurrentSchemaVersion = "v2"

// Load parses and validates an IR document from disk.
func Load(path string) (Document, error) {
	cleanPath := filepath.Clean(path)
	// #nosec G304 -- path is an explicit CLI/task input by design.
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return Document{}, fmt.Errorf("read ir file: %w", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(content)))
	dec.DisallowUnknownFields()

	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode ir json: %w", err)
	}
	if err := Validate(doc); err != nil {
		return Document{}, err
	}
	if err := Normalize(&doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

// Validate checks required fields and uniqueness constraints.
func Validate(doc Document) error {
	if strings.TrimSpace(doc.SchemaVersion) == "" {
		return fmt.Errorf("schema_version is required")
	}
	if doc.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", doc.SchemaVersion)
	}
	if err := ValidateBasePath(doc.API.BasePath); err != nil {
		return err
	}
	if strings.TrimSpace(doc.Info.Title) == "" {
		return fmt.Errorf("info.title is required")
	}
	if strings.TrimSpace(doc.Info.Version) == "" {
		return fmt.Errorf("info.version is required")
	}
	if len(doc.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint is required")
	}

	seenOperation := make(map[string]struct{}, len(doc.Endpoints))
	seenRoute := make(map[string]struct{}, len(doc.Endpoints))
	seenCLI := make(map[string]string, len(doc.Endpoints))
	commandPaths := make(map[string][]string, len(doc.Endpoints))
	for _, endpoint := range doc.Endpoints {
		if strings.TrimSpace(endpoint.Method) == "" {
			return fmt.Errorf("endpoint method is required")
		}
		if strings.TrimSpace(endpoint.Path) == "" {
			return fmt.Errorf("endpoint path is required")
		}
		if !strings.HasPrefix(strings.TrimSpace(endpoint.Path), "/") {
			return fmt.Errorf("endpoint %q path must start with \"/\"", endpoint.OperationID)
		}
		if strings.TrimSpace(endpoint.OperationID) == "" {
			return fmt.Errorf("endpoint operation_id is required")
		}
		if err := validateEndpointExtensions(endpoint.Extensions, fmt.Sprintf("endpoint %q", endpoint.OperationID)); err != nil {
			return err
		}
		for _, parameter := range endpoint.Parameters {
			if err := validateParameterSchema(doc, endpoint, parameter); err != nil {
				return err
			}
		}
		if endpoint.RequestBody != nil {
			if err := validateRequestBodySchema(doc, endpoint); err != nil {
				return err
			}
		}
		if len(endpoint.Responses) == 0 {
			return fmt.Errorf("endpoint %q must have at least one response", endpoint.OperationID)
		}
		seenResponseStatus := make(map[int]struct{}, len(endpoint.Responses))
		for _, response := range endpoint.Responses {
			if err := validateResponseExtensions(response.Extensions, fmt.Sprintf("endpoint %q response %d", endpoint.OperationID, response.StatusCode)); err != nil {
				return err
			}
			if response.StatusCode <= 0 {
				return fmt.Errorf("endpoint %q has invalid response status_code %d", endpoint.OperationID, response.StatusCode)
			}
			if _, exists := seenResponseStatus[response.StatusCode]; exists {
				return fmt.Errorf("endpoint %q has duplicate response status_code %d", endpoint.OperationID, response.StatusCode)
			}
			seenResponseStatus[response.StatusCode] = struct{}{}
			if strings.TrimSpace(response.Description) == "" {
				return fmt.Errorf("endpoint %q response %d description is required", endpoint.OperationID, response.StatusCode)
			}
			if shape, ok, err := ResponseShapeMetadata(response); err != nil {
				return fmt.Errorf("endpoint %q response %d shape metadata: %w", endpoint.OperationID, response.StatusCode, err)
			} else if ok {
				switch shape.Kind {
				case "wrapped_json":
					if shape.BodyType == "" {
						return fmt.Errorf("endpoint %q response %d wrapped_json body_type is required", endpoint.OperationID, response.StatusCode)
					}
				default:
					return fmt.Errorf("endpoint %q response %d has unsupported shape kind %q", endpoint.OperationID, response.StatusCode, shape.Kind)
				}
			}
			seenHeaders := make(map[string]struct{}, len(response.Headers))
			for _, header := range response.Headers {
				name := strings.TrimSpace(header.Name)
				if name == "" {
					return fmt.Errorf("endpoint %q response %d header name is required", endpoint.OperationID, response.StatusCode)
				}
				if err := validateSchemaRefExists(doc, header.Schema, fmt.Sprintf("endpoint %q response %d header %q", endpoint.OperationID, response.StatusCode, header.Name)); err != nil {
					return err
				}
				if _, exists := seenHeaders[strings.ToLower(name)]; exists {
					return fmt.Errorf("endpoint %q response %d has duplicate header %q", endpoint.OperationID, response.StatusCode, header.Name)
				}
				seenHeaders[strings.ToLower(name)] = struct{}{}
			}
			for idx, content := range response.Contents {
				if err := validateBodyContent(doc, content, fmt.Sprintf("endpoint %q response %d contents[%d]", endpoint.OperationID, response.StatusCode, idx)); err != nil {
					return err
				}
			}
		}

		normalizedCLI, err := normalizeEndpointCLI(doc, endpoint)
		if err != nil {
			return err
		}
		if err := validateEndpointCLI(doc, endpoint, normalizedCLI); err != nil {
			return err
		}
		if normalizedCLI != nil {
			command := CLICommandString(normalizedCLI)
			if existing, ok := seenCLI[command]; ok {
				return fmt.Errorf("duplicate cli.command %q for operations %q and %q", command, existing, endpoint.OperationID)
			}
			for other, otherPath := range commandPaths {
				if commandPathPrefix(normalizedCLI.Command, otherPath) || commandPathPrefix(otherPath, normalizedCLI.Command) {
					return fmt.Errorf("cli.command %q for operation %q conflicts with %q for operation %q", command, endpoint.OperationID, other, seenCLI[other])
				}
			}
			seenCLI[command] = endpoint.OperationID
			commandPaths[command] = append([]string(nil), normalizedCLI.Command...)
		}

		opKey := endpoint.OperationID
		if _, exists := seenOperation[opKey]; exists {
			return fmt.Errorf("duplicate operation_id %q", opKey)
		}
		seenOperation[opKey] = struct{}{}

		routeKey := strings.ToLower(endpoint.Method) + " " + endpoint.Path
		if _, exists := seenRoute[routeKey]; exists {
			return fmt.Errorf("duplicate endpoint route %q", routeKey)
		}
		seenRoute[routeKey] = struct{}{}
	}

	for name, schema := range doc.Schemas {
		if err := validateSchemaDefinition(doc, name, schema); err != nil {
			return err
		}
		if len(schema.PropertyOrder) > 0 {
			for _, propertyName := range schema.PropertyOrder {
				if _, ok := schema.Properties[propertyName]; !ok {
					return fmt.Errorf("schema %q property_order references unknown property %q", name, propertyName)
				}
			}
		}
	}

	return nil
}

func validateEndpointExtensions(extensions map[string]any, context string) error {
	for key, value := range extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("%s extension %q must start with \"x-\"", context, key)
		}
		switch key {
		case "x-authz":
			if err := validateAuthzExtension(value); err != nil {
				return fmt.Errorf("%s extension %q: %w", context, key, err)
			}
		case "x-apigen-manual":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("%s extension %q: x-apigen-manual must be boolean", context, key)
			}
		default:
			if strings.HasPrefix(key, "x-apigen-") {
				return fmt.Errorf("%s unsupported APIGen-owned extension %q", context, key)
			}
		}
		if err := validateJSONCompatibleExtensionValue(value); err != nil {
			return fmt.Errorf("%s extension %q: %w", context, key, err)
		}
	}
	return nil
}

func validateResponseExtensions(extensions map[string]any, context string) error {
	for key, value := range extensions {
		if !strings.HasPrefix(key, "x-") {
			return fmt.Errorf("%s extension %q must start with \"x-\"", context, key)
		}
		if key != ResponseShapeExtensionKey {
			if strings.HasPrefix(key, "x-apigen-") {
				return fmt.Errorf("%s unsupported APIGen-owned extension %q", context, key)
			}
			return fmt.Errorf("%s unsupported response extension %q", context, key)
		}
		if err := validateJSONCompatibleExtensionValue(value); err != nil {
			return fmt.Errorf("%s extension %q: %w", context, key, err)
		}
	}
	return nil
}

func validateAuthzExtension(value any) error {
	extension, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("x-authz must be an object")
	}
	mode, ok := extension["mode"]
	if !ok {
		return fmt.Errorf("x-authz.mode is required")
	}
	if _, ok := mode.(string); !ok {
		return fmt.Errorf("x-authz.mode must be string")
	}
	return nil
}

func validateJSONCompatibleExtensionValue(value any) error {
	switch typed := value.(type) {
	case nil, string, bool:
		return nil
	case float64:
		if math.IsInf(typed, 0) || math.IsNaN(typed) {
			return fmt.Errorf("number must be finite")
		}
		return nil
	case float32:
		if math.IsInf(float64(typed), 0) || math.IsNaN(float64(typed)) {
			return fmt.Errorf("number must be finite")
		}
		return nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case []any:
		for _, item := range typed {
			if err := validateJSONCompatibleExtensionValue(item); err != nil {
				return err
			}
		}
		return nil
	case []string:
		return nil
	case map[string]any:
		for _, item := range typed {
			if err := validateJSONCompatibleExtensionValue(item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported JSON value type %T", value)
	}
}

func validateParameterSchema(doc Document, endpoint Endpoint, parameter Parameter) error {
	if parameter.In == "" {
		return fmt.Errorf("endpoint %q parameter %q location is required", endpoint.OperationID, parameter.Name)
	}

	schemaType, format, err := resolvedParameterSchemaType(doc, parameter.Schema, fmt.Sprintf("endpoint %q parameter %q", endpoint.OperationID, parameter.Name))
	if err != nil {
		return err
	}

	switch schemaType {
	case "string":
		if format == "date-time" || format == "" {
			return nil
		}
		return nil
	case "array":
		if parameter.In != "query" {
			return fmt.Errorf("endpoint %q parameter %q arrays are only supported in query parameters", endpoint.OperationID, parameter.Name)
		}
		itemType, itemFormat, err := resolvedParameterArrayItemType(doc, parameter.Schema, fmt.Sprintf("endpoint %q parameter %q", endpoint.OperationID, parameter.Name))
		if err != nil {
			return err
		}
		switch itemType {
		case "string":
			if itemFormat == "" || itemFormat == "date-time" {
				return nil
			}
			return nil
		case "boolean", "bool":
			return nil
		case "integer":
			switch itemFormat {
			case "", "int32", "int64":
				return nil
			default:
				return fmt.Errorf("endpoint %q parameter %q has unsupported array item integer format %q", endpoint.OperationID, parameter.Name, itemFormat)
			}
		default:
			return fmt.Errorf("endpoint %q parameter %q has unsupported array item schema type %q", endpoint.OperationID, parameter.Name, itemType)
		}
	case "boolean", "bool":
		return nil
	case "integer":
		switch format {
		case "", "int32", "int64":
			return nil
		default:
			return fmt.Errorf("endpoint %q parameter %q has unsupported integer format %q", endpoint.OperationID, parameter.Name, format)
		}
	default:
		return fmt.Errorf("endpoint %q parameter %q has unsupported schema type %q", endpoint.OperationID, parameter.Name, schemaType)
	}
}

func validateRequestBodySchema(doc Document, endpoint Endpoint) error {
	if endpoint.RequestBody == nil {
		return nil
	}
	if len(endpoint.RequestBody.Contents) == 0 {
		return fmt.Errorf("endpoint %q request_body must declare at least one content", endpoint.OperationID)
	}
	for idx, content := range endpoint.RequestBody.Contents {
		if err := validateBodyContent(doc, content, fmt.Sprintf("endpoint %q request_body contents[%d]", endpoint.OperationID, idx)); err != nil {
			return err
		}
	}
	return nil
}

func validateBodyContent(doc Document, content BodyContent, context string) error {
	if strings.TrimSpace(content.ContentType) == "" {
		return fmt.Errorf("%s content_type is required", context)
	}
	switch content.BodyKind {
	case "json", "text", "binary", "file", "form_urlencoded", "multipart":
	default:
		return fmt.Errorf("%s has unsupported body_kind %q", context, content.BodyKind)
	}
	if content.Schema != nil {
		if err := validateSchemaRefExists(doc, *content.Schema, context+" schema"); err != nil {
			return err
		}
	}
	for idx, schemaRef := range content.AnyOf {
		if err := validateSchemaRefExists(doc, schemaRef, fmt.Sprintf("%s any_of[%d]", context, idx)); err != nil {
			return err
		}
	}
	for idx, part := range content.Parts {
		if strings.TrimSpace(part.Name) == "" {
			return fmt.Errorf("%s parts[%d] name is required", context, idx)
		}
		if strings.TrimSpace(part.BodyKind) == "" {
			return fmt.Errorf("%s parts[%d] body_kind is required", context, idx)
		}
		switch part.BodyKind {
		case "json", "text", "binary", "file":
		default:
			return fmt.Errorf("%s parts[%d] has unsupported body_kind %q", context, idx, part.BodyKind)
		}
		if part.Schema != nil {
			if err := validateSchemaRefExists(doc, *part.Schema, fmt.Sprintf("%s parts[%d] schema", context, idx)); err != nil {
				return err
			}
		}
	}
	switch content.BodyKind {
	case "multipart":
		if len(content.Parts) == 0 {
			return fmt.Errorf("%s multipart content must declare parts", context)
		}
	case "json", "text", "binary", "file", "form_urlencoded":
		if content.Schema == nil && len(content.AnyOf) == 0 {
			return fmt.Errorf("%s %s content must declare schema or any_of", context, content.BodyKind)
		}
	}
	return nil
}

func validateSchemaDefinition(doc Document, name string, schema Schema) error {
	if strings.TrimSpace(schema.Type) == "" {
		return fmt.Errorf("schema %q type is required", name)
	}
	for propertyName, property := range schema.Properties {
		if err := validateSchemaRefExists(doc, property.Schema, fmt.Sprintf("schema %q property %q", name, propertyName)); err != nil {
			return err
		}
	}
	if schema.Items != nil {
		if err := validateSchemaRefExists(doc, *schema.Items, fmt.Sprintf("schema %q items", name)); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaRefExists(doc Document, schemaRef SchemaRef, context string) error {
	if schemaRef.Ref != "" {
		name, ok := NormalizedSchemaRefName(schemaRef)
		if !ok {
			return fmt.Errorf("%s has invalid schema ref %q", context, schemaRef.Ref)
		}
		if _, ok := doc.Schemas[name]; !ok {
			return fmt.Errorf("%s references unknown schema %q", context, name)
		}
	}
	if schemaRef.Items != nil {
		if err := validateSchemaRefExists(doc, *schemaRef.Items, context+" items"); err != nil {
			return err
		}
	}
	if schemaRef.AdditionalProperties != nil && schemaRef.AdditionalProperties.Schema != nil {
		if err := validateSchemaRefExists(doc, *schemaRef.AdditionalProperties.Schema, context+" additional_properties"); err != nil {
			return err
		}
	}
	return nil
}

func resolvedParameterSchemaType(doc Document, schemaRef SchemaRef, context string) (string, string, error) {
	if schemaRef.Ref != "" {
		schema, ok := ResolveSchema(doc, schemaRef)
		if !ok {
			name, _ := NormalizedSchemaRefName(schemaRef)
			return "", "", fmt.Errorf("%s references unknown schema %q", context, name)
		}
		return strings.ToLower(strings.TrimSpace(schema.Type)), "", nil
	}
	return strings.ToLower(strings.TrimSpace(schemaRef.Type)), strings.ToLower(strings.TrimSpace(schemaRef.Format)), nil
}

func resolvedParameterArrayItemType(doc Document, schemaRef SchemaRef, context string) (string, string, error) {
	if schemaRef.Ref != "" {
		schema, ok := ResolveSchema(doc, schemaRef)
		if !ok {
			name, _ := NormalizedSchemaRefName(schemaRef)
			return "", "", fmt.Errorf("%s references unknown schema %q", context, name)
		}
		if schema.Items == nil {
			return "", "", fmt.Errorf("%s array schema must declare items", context)
		}
		return resolvedParameterSchemaType(doc, *schema.Items, context+" items")
	}
	if schemaRef.Items == nil {
		return "", "", fmt.Errorf("%s array schema must declare items", context)
	}
	return resolvedParameterSchemaType(doc, *schemaRef.Items, context+" items")
}

// Normalize applies deterministic ordering for generation.
func Normalize(doc *Document) error {
	sort.Slice(doc.Endpoints, func(i, j int) bool {
		if doc.Endpoints[i].Path == doc.Endpoints[j].Path {
			return strings.ToLower(doc.Endpoints[i].Method) < strings.ToLower(doc.Endpoints[j].Method)
		}
		return doc.Endpoints[i].Path < doc.Endpoints[j].Path
	})
	for i := range doc.Endpoints {
		normalizedCLI, err := normalizeEndpointCLI(*doc, doc.Endpoints[i])
		if err != nil {
			return err
		}
		doc.Endpoints[i].CLI = normalizedCLI
		for j := range doc.Endpoints[i].Parameters {
			if doc.Endpoints[i].Parameters[j].In == "query" && doc.Endpoints[i].Parameters[j].Explode == nil {
				explode := false
				doc.Endpoints[i].Parameters[j].Explode = &explode
			}
		}
		sort.Slice(doc.Endpoints[i].Responses, func(a, b int) bool {
			return doc.Endpoints[i].Responses[a].StatusCode < doc.Endpoints[i].Responses[b].StatusCode
		})
		for j := range doc.Endpoints[i].Responses {
			sort.Slice(doc.Endpoints[i].Responses[j].Headers, func(a, b int) bool {
				return strings.ToLower(doc.Endpoints[i].Responses[j].Headers[a].Name) < strings.ToLower(doc.Endpoints[i].Responses[j].Headers[b].Name)
			})
		}
	}
	return nil
}

func validateEndpointCLI(doc Document, endpoint Endpoint, cli *CLI) error {
	if cli == nil {
		return nil
	}
	if len(cli.Command) == 0 {
		return fmt.Errorf("endpoint %q cli.command is required when cli is present", endpoint.OperationID)
	}
	for _, segment := range cli.Command {
		if strings.TrimSpace(segment) == "" {
			return fmt.Errorf("endpoint %q cli.command must not contain empty segments", endpoint.OperationID)
		}
	}

	switch cli.BodyInput {
	case "none", "json", "flags", "flags_or_json", "text", "binary", "file":
	default:
		return fmt.Errorf("endpoint %q cli.body_input has unsupported value %q", endpoint.OperationID, cli.BodyInput)
	}

	switch cli.Confirm {
	case "none", "always":
	default:
		return fmt.Errorf("endpoint %q cli.confirm has unsupported value %q", endpoint.OperationID, cli.Confirm)
	}

	bodySchema, hasBodySchema := ResolveRequestBodySchema(doc, endpoint)
	bodyContent, hasBodyContent := PrimaryRequestBodyContent(endpoint)
	if endpoint.RequestBody == nil && cli.BodyInput != "none" {
		return fmt.Errorf("endpoint %q cli.body_input=%q requires request_body", endpoint.OperationID, cli.BodyInput)
	}
	if endpoint.RequestBody != nil && (cli.BodyInput == "flags" || cli.BodyInput == "flags_or_json") && (!hasBodySchema || bodySchema.Type != "object") {
		return fmt.Errorf("endpoint %q cli.body_input=%q requires an object request_body schema", endpoint.OperationID, cli.BodyInput)
	}
	if hasBodyContent && bodyContent.BodyKind == "multipart" && cli.BodyInput != "none" {
		return fmt.Errorf("endpoint %q multipart request bodies are not supported by generated CLI", endpoint.OperationID)
	}

	parametersByLocation := map[string]map[string]struct{}{
		"path":  {},
		"query": {},
		"body":  {},
	}
	for _, parameter := range endpoint.Parameters {
		parametersByLocation[parameter.In][parameter.Name] = struct{}{}
	}
	if hasBodySchema && bodySchema.Type == "object" {
		for name := range bodySchema.Properties {
			parametersByLocation["body"][name] = struct{}{}
		}
	}

	seenArgs := make(map[string]struct{}, len(cli.Args))
	for _, arg := range cli.Args {
		switch arg.Source {
		case "path", "query", "body":
		default:
			return fmt.Errorf("endpoint %q cli.args source %q is unsupported", endpoint.OperationID, arg.Source)
		}
		if strings.TrimSpace(arg.Name) == "" {
			return fmt.Errorf("endpoint %q cli.args name is required", endpoint.OperationID)
		}
		key := arg.Source + ":" + arg.Name
		if _, ok := seenArgs[key]; ok {
			return fmt.Errorf("endpoint %q cli.args contains duplicate binding %q", endpoint.OperationID, key)
		}
		seenArgs[key] = struct{}{}
		if _, ok := parametersByLocation[arg.Source][arg.Name]; !ok {
			return fmt.Errorf("endpoint %q cli.args references unknown %s field %q", endpoint.OperationID, arg.Source, arg.Name)
		}
		if arg.Source == "body" && !(cli.BodyInput == "flags" || cli.BodyInput == "flags_or_json") {
			return fmt.Errorf("endpoint %q cli.args body binding %q requires cli.body_input=flags or flags_or_json", endpoint.OperationID, arg.Name)
		}
	}

	if cli.Output == nil {
		return nil
	}
	switch cli.Output.Mode {
	case "detail", "collection", "empty", "raw":
	default:
		return fmt.Errorf("endpoint %q cli.output.mode has unsupported value %q", endpoint.OperationID, cli.Output.Mode)
	}

	successResponse, ok := SuccessResponse(endpoint)
	if !ok {
		return fmt.Errorf("endpoint %q cli output requires a success response", endpoint.OperationID)
	}
	successSchema, hasSuccessSchema := ResolveResponseBodySchema(doc, *successResponse)
	switch cli.Output.Mode {
	case "collection":
		itemSchema, err := validateCLICollectionSchema(doc, endpoint.OperationID, successSchema, hasSuccessSchema, cli)
		if err != nil {
			return err
		}
		for _, name := range cli.Output.TableColumns {
			if _, ok := itemSchema.Properties[name]; !ok {
				return fmt.Errorf("endpoint %q cli.output.table_columns references unknown item field %q", endpoint.OperationID, name)
			}
		}
		for _, name := range cli.Output.QuietFields {
			if _, ok := itemSchema.Properties[name]; !ok {
				return fmt.Errorf("endpoint %q cli.output.quiet_fields references unknown item field %q", endpoint.OperationID, name)
			}
		}
	case "detail":
		if !hasSuccessSchema || successSchema.Type != "object" {
			return fmt.Errorf("endpoint %q cli.output.mode=detail requires an object success schema", endpoint.OperationID)
		}
		for _, name := range append(append([]string(nil), cli.Output.TableColumns...), cli.Output.QuietFields...) {
			if _, ok := successSchema.Properties[name]; !ok {
				return fmt.Errorf("endpoint %q cli.output references unknown response field %q", endpoint.OperationID, name)
			}
		}
	case "empty", "raw":
		if cli.Pagination != nil {
			return fmt.Errorf("endpoint %q cli.pagination requires cli.output.mode=collection", endpoint.OperationID)
		}
	}

	if cli.Pagination != nil && cli.Output.Mode != "collection" {
		return fmt.Errorf("endpoint %q cli.pagination requires cli.output.mode=collection", endpoint.OperationID)
	}

	return nil
}

func validateCLICollectionSchema(doc Document, operationID string, successSchema Schema, hasSuccessSchema bool, cli *CLI) (Schema, error) {
	if !hasSuccessSchema || successSchema.Type != "object" {
		return Schema{}, fmt.Errorf("endpoint %q cli.output.mode=collection requires an object success schema", operationID)
	}
	itemsField := "data"
	if cli.Pagination != nil && strings.TrimSpace(cli.Pagination.ItemsField) != "" {
		itemsField = cli.Pagination.ItemsField
	}
	property, ok := successSchema.Properties[itemsField]
	if !ok {
		return Schema{}, fmt.Errorf("endpoint %q cli collection items field %q is missing", operationID, itemsField)
	}
	itemSchemaRef := property.Schema
	itemType := strings.ToLower(strings.TrimSpace(itemSchemaRef.Type))
	if itemType != "array" {
		return Schema{}, fmt.Errorf("endpoint %q cli collection items field %q must be an array", operationID, itemsField)
	}
	if itemSchemaRef.Items == nil {
		return Schema{}, fmt.Errorf("endpoint %q cli collection items field %q must declare items", operationID, itemsField)
	}
	itemSchema, ok := ResolveSchema(doc, *itemSchemaRef.Items)
	if !ok {
		return Schema{}, fmt.Errorf("endpoint %q cli collection item schema could not be resolved", operationID)
	}
	itemSchema.Type = strings.ToLower(strings.TrimSpace(itemSchema.Type))
	return itemSchema, nil
}

func normalizeEndpointCLI(doc Document, endpoint Endpoint) (*CLI, error) {
	cli := CloneCLI(endpoint.CLI)
	if cli == nil {
		return nil, nil
	}
	for i := range cli.Command {
		cli.Command[i] = strings.TrimSpace(cli.Command[i])
	}
	if endpoint.RequestBody == nil && cli.BodyInput == "" {
		cli.BodyInput = "none"
	}
	if endpoint.RequestBody != nil && cli.BodyInput == "" {
		content, ok := PrimaryRequestBodyContent(endpoint)
		switch {
		case !ok:
			cli.BodyInput = "none"
		case content.BodyKind == "json":
			requestBodySchema, ok := ResolveRequestBodySchema(doc, endpoint)
			if ok && strings.EqualFold(requestBodySchema.Type, "object") {
				cli.BodyInput = "flags_or_json"
			} else {
				cli.BodyInput = "json"
			}
		case content.BodyKind == "form_urlencoded":
			cli.BodyInput = "flags"
		case content.BodyKind == "text":
			cli.BodyInput = "text"
		case content.BodyKind == "binary":
			cli.BodyInput = "binary"
		case content.BodyKind == "file":
			cli.BodyInput = "file"
		default:
			cli.BodyInput = "json"
		}
	}
	if cli.Confirm == "" {
		if strings.EqualFold(endpoint.Method, "DELETE") {
			cli.Confirm = "always"
		} else {
			cli.Confirm = "none"
		}
	}
	if len(cli.Args) == 0 {
		cli.Args = defaultCLIArgs(doc, endpoint, cli.BodyInput)
	}
	output, pagination := deriveDefaultCLIOutput(doc, endpoint)
	if cli.Output == nil {
		cli.Output = output
	} else if output != nil {
		if cli.Output.Mode == "" {
			cli.Output.Mode = output.Mode
		}
		if len(cli.Output.TableColumns) == 0 {
			cli.Output.TableColumns = append([]string(nil), output.TableColumns...)
		}
		if len(cli.Output.QuietFields) == 0 {
			cli.Output.QuietFields = append([]string(nil), output.QuietFields...)
		}
	}
	if cli.Pagination == nil {
		cli.Pagination = pagination
	}
	return cli, nil
}

func defaultCLIArgs(doc Document, endpoint Endpoint, _ string) []CLIArg {
	pathArgs := defaultPositionalPathArgs(endpoint)
	args := make([]CLIArg, 0, len(pathArgs)+1)
	for _, name := range pathArgs {
		args = append(args, CLIArg{Source: "path", Name: name, DisplayName: strings.ReplaceAll(name, "_", "-")})
	}
	if shouldDefaultBodyNameArg(doc, endpoint) {
		args = append(args, CLIArg{Source: "body", Name: "name", DisplayName: "name"})
	}
	return args
}

func defaultPositionalPathArgs(endpoint Endpoint) []string {
	pathParams := PathParameterNames(endpoint.Path)
	if len(pathParams) == 0 {
		return nil
	}
	if strings.HasPrefix(endpoint.OperationID, "create") {
		selected := make([]string, 0, len(pathParams))
		for _, name := range pathParams {
			if name == "catalog_name" {
				continue
			}
			selected = append(selected, name)
		}
		return selected
	}
	if strings.HasPrefix(endpoint.OperationID, "list") {
		return append([]string(nil), pathParams...)
	}
	selected := make([]string, 0, len(pathParams))
	for _, name := range pathParams {
		if name == "catalog_name" {
			continue
		}
		selected = append(selected, name)
	}
	if len(selected) == 0 {
		selected = append(selected, pathParams[len(pathParams)-1])
	}
	return selected
}

func shouldDefaultBodyNameArg(doc Document, endpoint Endpoint) bool {
	if !strings.HasPrefix(endpoint.OperationID, "create") {
		return false
	}
	pathParams := PathParameterNames(endpoint.Path)
	if len(pathParams) == 0 {
		return false
	}
	for _, name := range pathParams {
		if name != "catalog_name" {
			return false
		}
	}
	bodySchema, ok := ResolveRequestBodySchema(doc, endpoint)
	if !ok || bodySchema.Type != "object" {
		return false
	}
	if _, ok := bodySchema.Properties["name"]; !ok {
		return false
	}
	for _, name := range bodySchema.Required {
		if name == "name" {
			return true
		}
	}
	return false
}

func deriveDefaultCLIOutput(doc Document, endpoint Endpoint) (*CLIOutput, *CLIPagination) {
	successResponse, ok := SuccessResponse(endpoint)
	if !ok {
		return nil, nil
	}
	if strings.EqualFold(endpoint.Method, "DELETE") || successResponse.StatusCode == 204 {
		return &CLIOutput{Mode: "empty"}, nil
	}
	successSchema, ok := ResolveResponseBodySchema(doc, *successResponse)
	if !ok {
		return &CLIOutput{Mode: "raw"}, nil
	}
	if successSchema.Type == "object" {
		if itemsProperty, ok := successSchema.Properties["data"]; ok && strings.EqualFold(itemsProperty.Schema.Type, "array") {
			output := &CLIOutput{Mode: "collection"}
			pagination := &CLIPagination{}
			pagination.ItemsField = "data"
			if nextProperty, ok := successSchema.Properties["next_page_token"]; ok && strings.EqualFold(nextProperty.Schema.Type, "string") {
				pagination.NextPageTokenField = "next_page_token"
			}
			if itemsProperty.Schema.Items != nil && itemsProperty.Schema.Items.Ref != "" {
				if itemSchema, ok := ResolveSchema(doc, *itemsProperty.Schema.Items); ok && itemSchema.Type == "object" {
					output.TableColumns = OrderedPropertyNames(itemSchema)
					output.QuietFields = defaultQuietFields(itemSchema)
				}
			}
			return output, pagination
		}
		return &CLIOutput{
			Mode:        "detail",
			QuietFields: defaultQuietFields(successSchema),
		}, nil
	}
	return &CLIOutput{Mode: "raw"}, nil
}

func defaultQuietFields(schema Schema) []string {
	fields := make([]string, 0, 3)
	for _, name := range []string{"id", "name", "key"} {
		if _, ok := schema.Properties[name]; ok {
			fields = append(fields, name)
		}
	}
	return fields
}

func commandPathPrefix(shorter []string, longer []string) bool {
	if len(shorter) >= len(longer) {
		return false
	}
	for i := range shorter {
		if shorter[i] != longer[i] {
			return false
		}
	}
	return true
}
