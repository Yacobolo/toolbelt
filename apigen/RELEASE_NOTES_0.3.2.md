# APIGen 0.3.2 Release Notes

## Breaking Changes

- JSON IR moves from schema version `v1` to `v2`.
- Request and response bodies are no longer assumed to be JSON.
- `request_body.schema` and `request_body.content_type` are replaced by `request_body.contents[]`.
- `response.schema`, `response.content_type`, and `response.any_of` are replaced by `response.contents[]`.
- Generated non-JSON request and response types no longer use `JSONBody` or `JSONResponse` names.
- Same-status response variants are represented as one IR response with multiple ordered contents.
- Multi-content generated responses use media-specific concrete names such as `ApplicationJSONResponse` and `ApplicationOctetStreamResponse`.
- `GenericRequest` inference is removed; TypeSpec contracts must name the schema they want generated.

## TypeSpec-Native HTTP

APIGen now follows resolved `@typespec/http` semantics for:

- JSON bodies
- `text/plain`
- `application/octet-stream`
- `Http.File`
- `application/x-www-form-urlencoded`
- `multipart/form-data`
- optional request bodies
- multiple response content variants
- standard response helpers such as `Response<Status>`, `Body<T>`, `OkResponse`, `CreatedResponse`, and `NoContentResponse`
- aliased response unions
- namespace/interface route containers

Vendor extensions remain available for application-specific metadata, but standard HTTP wire contracts should use TypeSpec HTTP constructs instead of custom raw-body extensions.

## Migration

Before:

```json
{
  "request_body": {
    "required": true,
    "content_type": "application/json",
    "schema": {"ref": "CreateWidgetRequest"}
  },
  "responses": [{
    "status_code": 200,
    "description": "ok",
    "schema": {"ref": "Widget"}
  }]
}
```

After:

```json
{
  "request_body": {
    "required": true,
    "contents": [{
      "content_type": "application/json",
      "body_kind": "json",
      "schema": {"ref": "CreateWidgetRequest"}
    }]
  },
  "responses": [{
    "status_code": 200,
    "description": "ok",
    "contents": [{
      "content_type": "application/json",
      "body_kind": "json",
      "schema": {"ref": "Widget"}
    }]
  }]
}
```

Generated Go migration:

- Rename `Gen<Operation>JSONBody` usages to `Gen<Operation>Body`.
- Keep JSON response constructors named `Gen<Operation><Status>JSONResponse`.
- Use `Gen<Operation><Status>TextResponse`, `BinaryResponse`, or `FileResponse` for non-JSON responses.
- For multi-content statuses, use media-specific response constructors such as `GenGetArtifact200ApplicationJSONResponse` or `GenGetArtifact200ApplicationOctetStreamResponse`.
- Multipart request handlers receive `Gen<Operation>Body` aliased to a generated `Gen<Operation>MultipartBody` struct. Required parts are concrete fields; optional parts are pointers. JSON/form parts decode to generated schema types, text parts decode to `string`, and binary/file parts decode to `[]byte`.
- Replace `GenericRequest` wrappers with the concrete TypeSpec model name.

## Preferred TypeSpec

```typespec
using Http;

model OkJson<T> {
  ...OkResponse;
  ...Body<T>;
}

model RateLimited {
  ...Response<429>;
  ...Body<Error>;
}

alias CommonErrors = BadRequest | RateLimited;

@route("/artifacts")
namespace Artifacts {
  @route("/{id}/blob")
  @put
  op replaceBlob(
    @path id: string,
    @header contentType: "application/octet-stream",
    @body body: bytes,
  ): OkJson<Artifact> | CommonErrors;
}
```

LibreDash binary upload migration:

```typespec
// Before: JSON object plus app-specific raw-body extension.
model DeploymentArtifactUploadRequest {
  value: bytes;
}

@extension("x-libredash-dispatch", "raw-body")
op uploadDeploymentArtifact(@body body: DeploymentArtifactUploadRequest): UploadDeploymentArtifactOK | BadRequest | Unauthorized;

// After: standard TypeSpec HTTP transport.
alias UploadDeploymentArtifactOK = OkJson<DeploymentArtifactResponse>;
alias CommonErrors = BadRequest | Unauthorized;

@route("/api/v1")
namespace Deployments {
  @route("/workspaces/{workspace}/deployments/{deployment}/artifact")
  @put
  op uploadDeploymentArtifact(
    @path workspace: string,
    @path deployment: string,
    @header contentType: "application/octet-stream",
    @body body: bytes,
  ): UploadDeploymentArtifactOK | CommonErrors;
}
```

## Compatibility Note

APIGen v0.3.1 remains the pinned all-JSON release. Use v0.3.2 when you want TypeSpec-native HTTP transport semantics and are ready to regenerate/migrate generated Go code.
