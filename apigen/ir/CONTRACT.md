# JSON IR Contract (`v2`)

`github.com/Yacobolo/toolbelt/apigen/ir` defines the versioned JSON intermediate representation consumed by APIGen.

## Versioning

- Current supported version: `v2`
- The root document must contain `schema_version: "v2"`
- Breaking IR changes require a new schema version
- v0.3.2 intentionally breaks the v1 all-JSON body shape

## Root Document

Required fields:

- `schema_version`
- `info.title`
- `info.version`
- `endpoints` with at least one entry

Optional fields:

- `info.description`
- `servers`
- `tags`
- `schemas`
- `extensions`

## Endpoints

Each endpoint must define:

- `method`
- `path`
- `operation_id`
- `responses`

Endpoint routes are unique by `lower(method) + " " + path`.
`operation_id` values are unique across the document.
Each endpoint may contain at most one response entry per `status_code`; multiple media variants for the same status belong in that response's ordered `contents`.

Endpoint-level `extensions` preserve operation vendor metadata. Generic extensions must use OpenAPI-style `x-*` keys and JSON-compatible values.

APIGen-owned endpoint extensions in current consumers:

- `x-authz`
- `x-apigen-manual`

## Body Contents

Request and response bodies use ordered `contents` entries.

Each content entry defines:

- `content_type`
- `body_kind`

Supported `body_kind` values:

- `json`
- `text`
- `binary`
- `file`
- `form_urlencoded`
- `multipart`

Schema-bearing content uses `schema`. Multiple response alternatives may use `any_of`. Multipart content uses `parts`, where each part has a name, body kind, optional content type, and optional schema.

JSON `bytes` values are represented as `type: string`, `format: byte`. Raw binary/file payloads are represented as `type: string`, `format: binary`.

## Responses

Each response must define:

- `status_code`
- `description`

Optional fields:

- `headers`
- `contents`
- `extensions`

Supported APIGen-owned response extension:

- `x-apigen-response-shape`

Current supported response shape:

- `wrapped_json`
  - requires `body_type`
  - indicates the generated server should treat the response as a JSON wrapper whose body type is named explicitly by `body_type`

Response headers are unique case-insensitively per response.

## Schemas

`schemas` is a named registry used by both emitted OpenAPI and generated Go code.

`SchemaRef.ref` values are normalized against component-style paths and resolved against this registry.

JSON and urlencoded form object bodies intended for generated Go output should resolve to named schema entries in this registry. Text bodies generate `string`; binary and file bodies generate `[]byte`. Generators reject anonymous object bodies when they cannot be mapped to a stable generated Go type.

## Contract Roles

- JSON IR is the generator input contract
- canonical OpenAPI is the published API contract artifact
- TypeSpec HTTP semantics are the source of truth for body kind, content type, routes, status codes, and parameters
- canonical OpenAPI may carry repo-owned metadata extensions such as `x-authz`
