package ir

import (
	"fmt"
	"sort"
	"strings"
)

// NormalizedSchemaRefName resolves a schema ref to a registry key.
func NormalizedSchemaRefName(schema SchemaRef) (string, bool) {
	if schema.Ref == "" {
		return "", false
	}
	ref := strings.TrimSpace(schema.Ref)
	ref = strings.TrimPrefix(ref, "#/components/schemas/")
	ref = strings.TrimPrefix(ref, "#/schemas/")
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		ref = ref[idx+1:]
	}
	if ref == "" {
		return "", false
	}
	return ref, true
}

// ResolveSchema returns the concrete schema referenced by the schema ref.
func ResolveSchema(doc Document, schemaRef SchemaRef) (Schema, bool) {
	name, ok := NormalizedSchemaRefName(schemaRef)
	if !ok {
		return Schema{}, false
	}
	schema, ok := doc.Schemas[name]
	return schema, ok
}

// ResolveRequestBodySchemaName returns the concrete request body schema name when present.
func ResolveRequestBodySchemaName(doc Document, endpoint Endpoint) (string, bool) {
	content, ok := PrimaryRequestBodyContent(endpoint)
	if !ok || content.Schema == nil {
		return "", false
	}
	ref := *content.Schema
	name, ok := NormalizedSchemaRefName(ref)
	if !ok {
		return "", false
	}
	if _, ok := doc.Schemas[name]; !ok {
		return "", false
	}
	return name, true
}

// ResolveRequestBodySchema returns the concrete request body schema when present.
func ResolveRequestBodySchema(doc Document, endpoint Endpoint) (Schema, bool) {
	name, ok := ResolveRequestBodySchemaName(doc, endpoint)
	if !ok {
		return Schema{}, false
	}
	schema, ok := doc.Schemas[name]
	return schema, ok
}

// SuccessResponse returns the preferred success response for CLI generation.
func SuccessResponse(endpoint Endpoint) (*Response, bool) {
	var best *Response
	for i := range endpoint.Responses {
		response := &endpoint.Responses[i]
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			continue
		}
		if best == nil || response.StatusCode < best.StatusCode {
			best = response
		}
	}
	return best, best != nil
}

// ResolveResponseBodySchema returns the schema used for the CLI-visible success body.
func ResolveResponseBodySchema(doc Document, response Response) (Schema, bool) {
	if shape, ok, _ := ResponseShapeMetadata(response); ok && shape.Kind == "wrapped_json" && shape.BodyType != "" {
		schema, ok := doc.Schemas[shape.BodyType]
		return schema, ok
	}
	content, ok := PrimaryResponseContent(response)
	if !ok || content.Schema == nil {
		return Schema{}, false
	}
	return ResolveSchema(doc, *content.Schema)
}

// PrimaryRequestBodyContent returns the preferred content entry for generated
// server and CLI code paths that can use only one request representation.
func PrimaryRequestBodyContent(endpoint Endpoint) (BodyContent, bool) {
	if endpoint.RequestBody == nil || len(endpoint.RequestBody.Contents) == 0 {
		return BodyContent{}, false
	}
	return preferredContent(endpoint.RequestBody.Contents)
}

// PrimaryResponseContent returns the preferred content entry for generated
// server and CLI code paths that can use only one response representation.
func PrimaryResponseContent(response Response) (BodyContent, bool) {
	if len(response.Contents) == 0 {
		return BodyContent{}, false
	}
	return preferredContent(response.Contents)
}

func preferredContent(contents []BodyContent) (BodyContent, bool) {
	for _, content := range contents {
		if content.BodyKind == "json" {
			return content, true
		}
	}
	return contents[0], true
}

// JoinAPIPath combines a contract base path with an authored endpoint path.
func JoinAPIPath(basePath string, endpointPath string) string {
	basePath = strings.TrimSpace(basePath)
	endpointPath = strings.TrimSpace(endpointPath)

	if basePath == "/" {
		basePath = ""
	}
	if endpointPath == "" {
		endpointPath = "/"
	}
	if endpointPath == "/" {
		if basePath == "" {
			return "/"
		}
		return basePath
	}
	if basePath == "" {
		return endpointPath
	}
	return strings.TrimRight(basePath, "/") + endpointPath
}

// ValidateBasePath checks APIGen API base path formatting.
func ValidateBasePath(basePath string) error {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return fmt.Errorf("api.base_path is required")
	}
	if !strings.HasPrefix(basePath, "/") {
		return fmt.Errorf("api.base_path must start with \"/\"")
	}
	if basePath != "/" && strings.HasSuffix(basePath, "/") {
		return fmt.Errorf("api.base_path must not end with \"/\" unless it is exactly \"/\"")
	}
	return nil
}

// CLICommandString renders a CLI command path as a space-delimited string.
func CLICommandString(cli *CLI) string {
	if cli == nil {
		return ""
	}
	return strings.Join(cli.Command, " ")
}

// CloneCLI returns a deep copy of CLI metadata.
func CloneCLI(in *CLI) *CLI {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Command) > 0 {
		out.Command = append([]string(nil), in.Command...)
	}
	if len(in.Args) > 0 {
		out.Args = append([]CLIArg(nil), in.Args...)
	}
	if in.Output != nil {
		output := *in.Output
		if len(in.Output.TableColumns) > 0 {
			output.TableColumns = append([]string(nil), in.Output.TableColumns...)
		}
		if len(in.Output.QuietFields) > 0 {
			output.QuietFields = append([]string(nil), in.Output.QuietFields...)
		}
		out.Output = &output
	}
	if in.Pagination != nil {
		pagination := *in.Pagination
		out.Pagination = &pagination
	}
	return &out
}

// PathParameterNames extracts ordered "{param}" names from an endpoint path.
func PathParameterNames(path string) []string {
	params := make([]string, 0, strings.Count(path, "{"))
	for i := 0; i < len(path); i++ {
		if path[i] != '{' {
			continue
		}
		j := i + 1
		for j < len(path) && path[j] != '}' {
			j++
		}
		if j >= len(path) || j == i+1 {
			continue
		}
		params = append(params, path[i+1:j])
		i = j
	}
	return params
}

// OrderedPropertyNames returns a deterministic property order for a schema.
func OrderedPropertyNames(schema Schema) []string {
	if len(schema.Properties) == 0 {
		return nil
	}
	if len(schema.PropertyOrder) > 0 {
		names := make([]string, 0, len(schema.PropertyOrder))
		for _, name := range schema.PropertyOrder {
			if _, ok := schema.Properties[name]; ok {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func exportedName(value string) string {
	parts := splitIdentifier(value)
	if len(parts) == 0 {
		return "Operation"
	}
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func splitIdentifier(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	replacer := strings.NewReplacer("-", " ", "_", " ", ".", " ", "/", " ")
	value = replacer.Replace(value)
	chunks := strings.Fields(value)
	if len(chunks) > 0 {
		return chunks
	}
	return []string{value}
}
