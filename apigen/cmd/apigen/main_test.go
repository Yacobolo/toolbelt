package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	openapiemit "github.com/Yacobolo/toolbelt/apigen/emit/openapi"
	"github.com/Yacobolo/toolbelt/apigen/ir"
	"github.com/stretchr/testify/require"
)

func TestRunCLI_TopLevelHelp(t *testing.T) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"--help"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.Contains(t, stdout.String(), "Usage:")
	require.Contains(t, stdout.String(), "apigen <command> [flags]")
	require.Contains(t, stdout.String(), "typespec-compile")
	require.NotContains(t, stdout.String(), "cue-compile")
	require.NotContains(t, stdout.String(), "cue-bootstrap")
	require.Contains(t, stdout.String(), `Use "apigen <command> -h" for command-specific flags.`)
	require.Empty(t, stderr.String())
}

func TestRunCLI_NoArgsShowsUsage(t *testing.T) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI(nil, &stdout, &stderr)
	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "Usage:")
	require.Contains(t, stderr.String(), "apigen <command> [flags]")
}

func TestRunCLI_RemovedCUECommandsFailUnsupported(t *testing.T) {
	t.Helper()

	for _, command := range []string{"cue-compile", "cue-bootstrap"} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		code := runCLI([]string{command}, &stdout, &stderr)
		require.Equal(t, 1, code)
		require.Empty(t, stdout.String())
		require.Contains(t, stderr.String(), `unsupported command "`+command+`"`)
	}
}

func TestGenerateArtifacts(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	irPath := filepath.Join(dir, "ir.json")

	require.NoError(t, os.WriteFile(irPath, []byte(`{
  "schema_version": "v1",
  "api": {"base_path": "/v1"},
  "info": {"title": "Duck", "version": "0.1.0", "description": "test"},
  "servers": [{"url": "https://localhost:8080", "description": "local"}],
  "schemas": {
    "HealthResponse": {
      "type": "object",
      "properties": {
        "status": {"description": "Health state", "schema": {"type": "string"}}
      },
      "required": ["status"]
    }
  },
  "endpoints": [
    {
      "method": "get",
      "path": "/healthz",
      "operation_id": "getHealth",
      "summary": "Health check",
      "tags": ["system"],
      "responses": [{"status_code": 200, "description": "ok", "schema": {"ref": "HealthResponse"}}]
    }
  ]
}`), 0o644))

	doc, err := loadDocument(irPath)
	require.NoError(t, err)

	openapiPath := filepath.Join(dir, "openapi.yaml")
	serverPath := filepath.Join(dir, "server.apigen.gen.go")
	requestModelsPath := filepath.Join(dir, "request_models.gen.go")
	cliPath := filepath.Join(dir, "cli.gen.go")
	canonicalOpenAPIPath := filepath.Join(dir, "canonical-openapi.yaml")
	require.NoError(t, os.WriteFile(canonicalOpenAPIPath, []byte("openapi: 3.0.0\ninfo:\n  title: Duck\n  version: 0.1.0\npaths: {}\n"), 0o644))

	require.NoError(t, generateOpenAPI(doc, openapiPath))
	require.NoError(t, generateServer(doc, serverPath, "api", requestModelsPath, "api", canonicalOpenAPIPath))
	require.NoError(t, generateCLI(doc, cliPath, "gen"))

	_, err = os.Stat(openapiPath)
	require.NoError(t, err)
	_, err = os.Stat(serverPath)
	require.NoError(t, err)
	_, err = os.Stat(requestModelsPath)
	require.NoError(t, err)
	_, err = os.Stat(cliPath)
	require.NoError(t, err)
}

func TestResolveCommandConfig_GroupedManifestTarget(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
    cli_out:
      dir: cmd/cli/gen
`), 0o644))

	config, err := resolveCommandConfig("all", manifestPath, "example", commandConfig{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "api", "typespec"), config.TypeSpecDir)
	require.Equal(t, filepath.Join(dir, "api", "gen", "json-ir.json"), config.IRPath)
	require.Equal(t, filepath.Join(dir, "api", "gen", "openapi.yaml"), config.CanonicalOpenAPIPath)
	require.Equal(t, filepath.Join(dir, "internal", "api", "gen", "server.apigen.gen.go"), config.ServerOut)
	require.Equal(t, "gen", config.ServerPackage)
	require.Equal(t, filepath.Join(dir, "internal", "api", "gen", "request_models.gen.go"), config.RequestModelsOut)
	require.Equal(t, "gen", config.RequestModelsPackage)
	require.Equal(t, filepath.Join(dir, "cmd", "cli", "gen", "apigen_registry.gen.go"), config.CLIOut)
	require.Equal(t, "gen", config.CLIPackage)
	require.True(t, config.GenerateCLI)
}

func TestResolveCommandConfig_TypeSpecCompileRequiresTypeSpecDir(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
`), 0o644))

	config, err := resolveCommandConfig("typespec-compile", manifestPath, "example", commandConfig{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "api", "typespec"), config.TypeSpecDir)

	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
`), 0o644))
	_, err = resolveCommandConfig("typespec-compile", manifestPath, "example", commandConfig{})
	require.Error(t, err)
	require.ErrorContains(t, err, "typespec_dir")
}

func TestCompileTypeSpec_GeneratesIRAndOpenAPI(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	setupManagedTypeSpecCache(t)
	irPath := filepath.Join(dir, "json-ir.json")
	openAPIPath := filepath.Join(dir, "openapi.yaml")
	fixtureDir := filepath.Join("..", "..", "typespec", "test", "fixtures", "todo")

	require.NoError(t, compileTypeSpec(fixtureDir, irPath, openAPIPath))

	doc, err := loadDocument(irPath)
	require.NoError(t, err)
	require.Equal(t, "APIGen Todo Example", doc.Info.Title)
	require.Len(t, doc.Endpoints, 5)
	require.Equal(t, "CreateTodoRequest", doc.Endpoints[1].RequestBody.Schema.Ref)
	require.Equal(t, []string{"todos", "create"}, doc.Endpoints[1].CLI.Command)
	require.FileExists(t, openAPIPath)
}

func TestCompileTypeSpec_PreservesOutputsWhenToolchainUnavailable(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv(typeSpecPackageDirEnv, filepath.Join(dir, "missing-typespec-package"))
	irPath := filepath.Join(dir, "json-ir.json")
	openAPIPath := filepath.Join(dir, "openapi.yaml")
	require.NoError(t, os.WriteFile(irPath, []byte(`{"existing":true}`), 0o644))
	require.NoError(t, os.WriteFile(openAPIPath, []byte("existing: true\n"), 0o644))

	err := compileTypeSpec(filepath.Join("..", "..", "typespec", "test", "fixtures", "todo"), irPath, openAPIPath)
	require.Error(t, err)
	require.ErrorContains(t, err, "typespec compiler not found")
	require.Equal(t, `{"existing":true}`, strings.TrimSpace(mustReadString(t, irPath)))
	require.Equal(t, "existing: true", strings.TrimSpace(mustReadString(t, openAPIPath)))
}

func TestResolveTypeSpecPackage_UsesDevelopmentOverride(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv(typeSpecPackageDirEnv, dir)

	pkg, err := resolveTypeSpecPackage()
	require.NoError(t, err)
	require.Equal(t, mustAbs(t, dir), pkg.Dir)
	require.False(t, pkg.Managed)
}

func TestInstallBundledTypeSpecPackage_UsesWritableCache(t *testing.T) {
	t.Helper()

	cacheRoot := t.TempDir()
	pkg, err := installBundledTypeSpecPackage(cacheRoot)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(pkg.Dir, filepath.Join(cacheRoot, "apigen", "typespec")+string(os.PathSeparator)))
	require.True(t, pkg.Managed)
	require.FileExists(t, filepath.Join(pkg.Dir, "package.json"))
	require.FileExists(t, filepath.Join(pkg.Dir, "package-lock.json"))
	require.FileExists(t, filepath.Join(pkg.Dir, "lib", "main.tsp"))
	require.FileExists(t, filepath.Join(pkg.Dir, "dist", "src", "index.js"))
}

func TestCompileTypeSpec_FailurePreservesExistingOutputs(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	setupManagedTypeSpecCache(t)
	irPath := filepath.Join(dir, "json-ir.json")
	openAPIPath := filepath.Join(dir, "openapi.yaml")
	require.NoError(t, os.WriteFile(irPath, []byte(`{"stale":true}`), 0o644))
	require.NoError(t, os.WriteFile(openAPIPath, []byte("stale: true\n"), 0o644))

	fixtureDir := filepath.Join("..", "..", "typespec", "test", "fixtures", "invalid")
	err := compileTypeSpec(fixtureDir, irPath, openAPIPath)
	require.Error(t, err)
	require.ErrorContains(t, err, "requires request body to resolve to a named model schema")
	require.Equal(t, `{"stale":true}`, strings.TrimSpace(mustReadString(t, irPath)))
	require.Equal(t, "stale: true", strings.TrimSpace(mustReadString(t, openAPIPath)))
}

func TestResolveCommandConfig_GroupedManifestOverrides(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/generated/api
      package: transport
      server_file: service_server.gen.go
      request_models_file: models.gen.go
    cli_out:
      dir: internal/generated/commands
      package: cli
      file: registry.gen.go
`), 0o644))

	config, err := resolveCommandConfig("all", manifestPath, "example", commandConfig{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "internal", "generated", "api", "service_server.gen.go"), config.ServerOut)
	require.Equal(t, "transport", config.ServerPackage)
	require.Equal(t, filepath.Join(dir, "internal", "generated", "api", "models.gen.go"), config.RequestModelsOut)
	require.Equal(t, "transport", config.RequestModelsPackage)
	require.Equal(t, filepath.Join(dir, "internal", "generated", "commands", "registry.gen.go"), config.CLIOut)
	require.Equal(t, "cli", config.CLIPackage)
}

func TestResolveCommandConfig_GroupedManifestWithoutCLI(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
`), 0o644))

	config, err := resolveCommandConfig("all", manifestPath, "example", commandConfig{})
	require.NoError(t, err)
	require.False(t, config.GenerateCLI)
	require.Empty(t, config.CLIOut)

	_, err = resolveCommandConfig("cli", manifestPath, "example", commandConfig{})
	require.Error(t, err)
	require.ErrorContains(t, err, "cli_out")
}

func TestResolveCommandConfig_GroupedManifestRejectsLegacyFields(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    server_out: internal/api/server.apigen.gen.go
    go_out:
      dir: internal/api/gen
`), 0o644))

	_, err := resolveCommandConfig("all", manifestPath, "example", commandConfig{})
	require.Error(t, err)
	require.ErrorContains(t, err, "legacy flat manifest fields")
}

func TestResolveCommandConfig_GroupedManifestRejectsStringCLIOut(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/gen
    cli_out: cmd/cli/gen/apigen_registry.gen.go
`), 0o644))

	_, err := resolveCommandConfig("all", manifestPath, "example", commandConfig{})
	require.Error(t, err)
	require.ErrorContains(t, err, "cli_out must be a mapping")
}

func TestResolveCommandConfig_GroupedManifestRejectsInvalidInferredPackage(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: example
    typespec_dir: api/typespec
    ir_out: api/gen/json-ir.json
    openapi_out: api/gen/openapi.yaml
    go_out:
      dir: internal/api/123-generated
`), 0o644))

	_, err := resolveCommandConfig("all", manifestPath, "example", commandConfig{})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid inferred go package")
}

func TestMultiTargetManifest_GeneratesVersionedArtifacts(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	setupManagedTypeSpecCache(t)
	writeMinimalTypeSpecContract(t, filepath.Join(root, "api", "v1", "typespec"), "/v1", "Widget API", "1.0.0")
	writeMinimalTypeSpecContract(t, filepath.Join(root, "api", "v2", "typespec"), "/v2", "Widget API v2", "2.0.0")

	manifestPath := filepath.Join(root, "apigen.targets.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(`targets:
  - name: v1
    typespec_dir: api/v1/typespec
    ir_out: internal/api/v1/gen/json-ir.json
    openapi_out: internal/api/v1/gen/openapi.yaml
    go_out:
      dir: internal/api/v1
      package: apiv1
      server_file: server.apigen.gen.go
      request_models_file: request_models.gen.go
    cli_out:
      dir: pkg/cli/gen
      package: genv1
      file: apigen_v1_registry.gen.go
  - name: v2
    typespec_dir: api/v2/typespec
    ir_out: internal/api/v2/gen/json-ir.json
    openapi_out: internal/api/v2/gen/openapi.yaml
    go_out:
      dir: internal/api/v2
      package: apiv2
      server_file: server.apigen.gen.go
      request_models_file: request_models.gen.go
`), 0o644))

	v1Config, err := resolveCommandConfig("all", manifestPath, "v1", commandConfig{})
	require.NoError(t, err)
	require.NoError(t, compileTypeSpec(v1Config.TypeSpecDir, v1Config.IROut, v1Config.OpenAPIOut))

	v1Doc, err := loadDocument(v1Config.IRPath)
	require.NoError(t, err)
	require.Equal(t, "Widget API", v1Doc.Info.Title)
	require.NoError(t, generateServer(v1Doc, v1Config.ServerOut, v1Config.ServerPackage, v1Config.RequestModelsOut, v1Config.RequestModelsPackage, v1Config.CanonicalOpenAPIPath))
	require.NoError(t, generateCLI(v1Doc, v1Config.CLIOut, v1Config.CLIPackage))

	v1OpenAPI := mustReadString(t, v1Config.OpenAPIOut)
	require.Contains(t, v1OpenAPI, "/v1/widgets:")
	v1Server := mustReadString(t, v1Config.ServerOut)
	require.Contains(t, v1Server, `Path: "/v1/widgets"`)
	v1CLI := mustReadString(t, v1Config.CLIOut)
	require.Contains(t, v1CLI, `Path: "/v1/widgets"`)

	v2Config, err := resolveCommandConfig("all", manifestPath, "v2", commandConfig{})
	require.NoError(t, err)
	require.False(t, v2Config.GenerateCLI)
	require.NoError(t, compileTypeSpec(v2Config.TypeSpecDir, v2Config.IROut, v2Config.OpenAPIOut))

	v2Doc, err := loadDocument(v2Config.IRPath)
	require.NoError(t, err)
	require.Equal(t, "Widget API v2", v2Doc.Info.Title)
	require.NoError(t, generateServer(v2Doc, v2Config.ServerOut, v2Config.ServerPackage, v2Config.RequestModelsOut, v2Config.RequestModelsPackage, v2Config.CanonicalOpenAPIPath))

	v2OpenAPI := mustReadString(t, v2Config.OpenAPIOut)
	require.Contains(t, v2OpenAPI, "/v2/widgets:")
	v2Server := mustReadString(t, v2Config.ServerOut)
	require.Contains(t, v2Server, `Path: "/v2/widgets"`)
	_, err = os.Stat(v2Config.CLIOut)
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestGenerateServer_FailsForUnnamedRequestBodySchema(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	doc := ir.Document{
		SchemaVersion: "v1",
		API:           ir.API{BasePath: "/v1"},
		Info:          ir.Info{Title: "Widget API", Version: "1.0.0"},
		OpenAPI:       ir.OpenAPI{Version: "3.0.0"},
		Schemas: map[string]ir.Schema{
			"GenericRequest": {Type: "object"},
			"Widget": {
				Type: "object",
				Properties: map[string]ir.SchemaProperty{
					"id": {Schema: ir.SchemaRef{Type: "string"}},
				},
				Required: []string{"id"},
			},
		},
		Endpoints: []ir.Endpoint{
			{
				Method:      "post",
				Path:        "/widgets",
				OperationID: "createWidget",
				RequestBody: &ir.RequestBody{Schema: ir.SchemaRef{Ref: "GenericRequest"}},
				Responses:   []ir.Response{{StatusCode: 201, Description: "created", Schema: &ir.SchemaRef{Ref: "Widget"}}},
			},
		},
	}

	canonicalOpenAPIPath := writeCanonicalOpenAPI(t, dir, doc)
	serverPath := filepath.Join(dir, "server.apigen.gen.go")
	requestModelsPath := filepath.Join(dir, "request_models.gen.go")

	err := generateServer(doc, serverPath, "api", requestModelsPath, "api", canonicalOpenAPIPath)
	require.Error(t, err)
	require.ErrorContains(t, err, "generic request body schema could not be resolved")
	require.ErrorContains(t, err, "createWidget")
}

func writeMinimalTypeSpecContract(t *testing.T, typeSpecDir string, pathPrefix string, title string, version string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(typeSpecDir, 0o755))
	source := `using Http;
using TypeSpec.OpenAPI;

@service(#{ title: "` + title + `" })
@info(#{ version: "` + version + `" })
namespace WidgetAPI;

model Widget {
  id: string;
  name: string;
}

@route("` + pathPrefix + `/widgets")
@get
@summary("List widgets")
@apigen.cli(#{ command: #["widgets", "list"] })
op listWidgets(): Widget;
`
	require.NoError(t, os.WriteFile(filepath.Join(typeSpecDir, "main.tsp"), []byte(source), 0o644))
}

func mustReadString(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(content))
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	return abs
}

func setupManagedTypeSpecCache(t *testing.T) {
	t.Helper()

	t.Setenv(typeSpecPackageDirEnv, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
}

func writeCanonicalOpenAPI(t *testing.T, dir string, doc ir.Document) string {
	t.Helper()

	content, err := openapiemit.EmitYAML(doc, openapiemit.Options{})
	require.NoError(t, err)

	path := filepath.Join(dir, "canonical-openapi.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))
	return path
}
