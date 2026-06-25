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

Endpoint parameters support `path`, `query`, and `header` locations. Other locations, including `cookie`, are rejected by IR validation until all generated surfaces support them consistently.

Supported security schemes in v0.3.2 are HTTP Bearer auth and `ApiKeyAuth<ApiKeyLocation.header, "X-API-Key">`. Unsupported schemes fail closed before IR emission.

APIGen-owned endpoint extensions in current consumers:

- `x-authz`
- `x-apigen-manual`

## Body Contents

Request and response bodies use ordered `contents` entries. TypeSpec `@sharedRoute` and same-endpoint `@overload` declarations are coalesced before IR emission when they describe one compatible HTTP operation.

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

Schema-bearing content uses `schema`. Multiple response alternatives may use `any_of`. Multipart content uses `parts`. `SchemaRef.enum` is supported for inline string-literal parameter schemas such as coalesced `Accept` headers.

Within a single request body or response, `contents[].content_type` values must be unique case-insensitively. TypeSpec emission deduplicates identical duplicate variants, but rejects incompatible same-status variants that reuse the same media type.

JSON `bytes` values are represented as `type: string`, `format: byte`. Raw binary/file payloads are represented as `type: string`, `format: binary`.

Generated Go treats raw `bytes` as `[]byte`. TypeSpec `Http.File` uses generated `GenFile`, which supports both simple `Contents []byte` and streaming `Reader io.ReadCloser` payloads plus `ContentType`, optional `Filename`, and optional `Size` metadata.

## Multipart Parts

Each multipart part defines:

- `name`: generated field/property name
- `body_kind`: `json`, `text`, `binary`, or `file`
- optional `wire_name`: HTTP part name; omitted for unnamed `multipart/mixed` tuple parts
- optional `part_kind`: `model` or `tuple`
- optional `repeated`: true for repeated TypeSpec `HttpPart<T>[]` parts
- optional `required`
- optional `content_type`
- optional `filename`: true when a TypeSpec `Http.File` filename is available
- optional `schema`

Named `multipart/form-data` parts use `wire_name`. Unnamed `multipart/mixed` tuple parts are positional and keep stable generated names such as `part1`, `part2`. `HttpPart<T[]>` is a single JSON-array part; `HttpPart<T>[]` is repeated parts.

Generated CLI metadata includes ordered multipart part specs so runtime/cobra can accept repeated `--part name=value`, `--part name=@file`, or `--part name=-` flags. For `multipart/mixed`, tuple parts are addressed by generated names such as `part1` and `part2` and emitted in IR order.

Generated server decoding is strict. Unknown form-data part names, duplicate non-repeated form-data parts, and extra mixed tuple parts produce `400` responses. Repeated form-data parts are collected in request order for that part. Mixed tuple parts decode by wire order and do not use optional part names for reordering.

Canonical OpenAPI emits ordinary object schemas and `encoding` for `multipart/form-data`. For `multipart/mixed`, OpenAPI 3.0 receives best-effort schema output plus APIGen vendor metadata:

- `x-apigen-multipart-kind: "mixed"`
- ordered `x-apigen-multipart-parts`

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

JSON and urlencoded form object bodies intended for generated Go output should resolve to named schema entries in this registry. Text bodies generate `string`; raw binary `bytes` bodies generate `[]byte`; TypeSpec `Http.File` bodies generate streaming-capable `GenFile` with byte contents or a reader, content type, optional filename, and optional size metadata. Generators reject anonymous object bodies when they cannot be mapped to a stable generated Go type.

## Contract Roles

- JSON IR is the generator input contract
- canonical OpenAPI is the published API contract artifact
- TypeSpec HTTP semantics are the source of truth for body kind, content type, routes, status codes, and parameters
- canonical OpenAPI may carry repo-owned metadata extensions such as `x-authz`
