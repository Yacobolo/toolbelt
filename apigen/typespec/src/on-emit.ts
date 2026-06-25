import {
  emitFile,
  getAllTags,
  getDoc,
  getService,
  getSummary,
  isArrayModelType,
  isRecordModelType,
  walkPropertiesInherited,
  type EmitContext,
  type Enum,
  type Model,
  type ModelProperty,
  type Namespace,
  type Operation,
  type Program,
  type Scalar,
  type Type,
  type Union,
} from "@typespec/compiler";
import {
  getAllHttpServices,
  getServers,
  resolveAuthentication,
  type AuthenticationReference,
  type HttpAuth,
  type HttpAuthRef,
  type HttpOperation,
  type HttpOperationParameter,
  type HttpOperationPart,
  type HttpOperationResponse,
  type HttpOperationResponseContent,
  type HttpPayloadBody,
  type HttpService,
} from "@typespec/http";
import { getExtensions, getOperationId, getTagsMetadata, resolveInfo, resolveOperationId } from "@typespec/openapi";
import { getAuthz, getCLI, getResponseShape, isManual } from "./decorators.js";
import { type EmitterOptions, reportDiagnostic } from "./lib.js";

interface Document {
  schema_version: "v2";
  api: { base_path: string };
  info: { title: string; version: string; description?: string };
  openapi?: {
    version?: string;
    tag_order?: string[];
    security?: Record<string, string[]>[];
    security_schemes?: Record<string, SecurityScheme>;
  };
  servers?: Server[];
  tags?: Tag[];
  schemas?: Record<string, Schema>;
  endpoints: Endpoint[];
}

interface Server {
  url: string;
  description?: string;
}

interface Tag {
  name: string;
  description?: string;
}

interface SecurityScheme {
  type: string;
  in?: string;
  name?: string;
  scheme?: string;
}

interface Endpoint {
  method: string;
  path: string;
  operation_id: string;
  summary?: string;
  description?: string;
  tags?: string[];
  parameters?: Parameter[];
  request_body?: RequestBody;
  responses: Response[];
  cli?: unknown;
  security?: Record<string, string[]>[];
  extensions?: Record<string, unknown>;
}

interface DecoratorNodeLike {
  target?: MemberExpressionNodeLike | IdentifierNodeLike;
  arguments?: readonly ExtensionLiteralNodeLike[];
}

interface IdentifierNodeLike {
  sv?: unknown;
}

interface MemberExpressionNodeLike {
  id?: IdentifierNodeLike;
}

interface ExtensionLiteralNodeLike {
  value?: unknown;
  target?: IdentifierNodeLike;
  arguments?: readonly ExtensionLiteralNodeLike[];
  id?: IdentifierNodeLike;
  properties?: readonly ExtensionObjectPropertyNodeLike[];
  values?: readonly ExtensionLiteralNodeLike[];
}

interface ExtensionObjectPropertyNodeLike {
  id?: IdentifierNodeLike & { value?: unknown };
  value?: ExtensionLiteralNodeLike;
}

type LiteralConversionResult =
  | { ok: true; value: unknown }
  | { ok: false };

interface Parameter {
  name: string;
  in: string;
  required?: boolean;
  description?: string;
  explode?: boolean;
  schema: SchemaRef;
}

interface RequestBody {
  required?: boolean;
  description?: string;
  contents: BodyContent[];
}

interface Response {
  status_code: number;
  description: string;
  headers?: Header[];
  contents?: BodyContent[];
  extensions?: Record<string, unknown>;
}

interface BodyContent {
  content_type: string;
  body_kind: "json" | "text" | "binary" | "file" | "form_urlencoded" | "multipart";
  schema?: SchemaRef;
  any_of?: SchemaRef[];
  parts?: MultipartPart[];
}

interface MultipartPart {
  name: string;
  required?: boolean;
  description?: string;
  content_type?: string;
  body_kind?: "json" | "text" | "binary" | "file";
  schema?: SchemaRef;
}

interface Header {
  name: string;
  required?: boolean;
  description?: string;
  schema: SchemaRef;
}

interface Schema {
  type: string;
  title?: string;
  description?: string;
  properties?: Record<string, SchemaProperty>;
  property_order?: string[];
  required?: string[];
  items?: SchemaRef;
  enum?: string[];
}

interface SchemaProperty {
  description?: string;
  schema: SchemaRef;
}

interface SchemaRef {
  ref?: string;
  type?: string;
  format?: string;
  items?: SchemaRef;
  additional_properties?: { any?: boolean; schema?: SchemaRef };
}

class IRBuilder {
  readonly schemas = new Map<string, Model>();
  readonly enums = new Map<string, Enum>();
  private readonly emittedSchemas = new Set<string>();
  private readonly emittedEnums = new Set<string>();
  private failed = false;

  constructor(readonly program: Program) {}

  hasFailed() {
    return this.failed;
  }

  schemaRef(type: Type, context: string): SchemaRef {
    if (type.kind === "Model") {
      if (isArrayModelType(type)) {
        return { type: "array", items: this.schemaRef(type.indexer.value, `${context} items`) };
      }
      if (isRecordModelType(type)) {
        return {
          type: "object",
          additional_properties: { schema: this.schemaRef(type.indexer.value, `${context} value`) },
        };
      }
      if (isNamedUserModel(type)) {
        this.schemas.set(type.name, type);
        return { ref: type.name };
      }
      return this.inlineObjectRef(type, context);
    }
    if (type.kind === "Scalar") {
      return scalarSchemaRef(type);
    }
    if (type.kind === "Enum") {
      if (type.name !== "") {
        this.enums.set(type.name, type);
        return { ref: type.name };
      }
      this.unsupported(type, context);
      return { type: "string" };
    }
    if (type.kind === "Union") {
      const enumValuesForUnion = stringLiteralUnionValues(type);
      if (enumValuesForUnion) {
        this.report("unsupported-inline-enum", { context }, type);
        return { type: "string" };
      }
    }
    if (type.kind === "String") {
      return { type: "string" };
    }
    if (type.kind === "Boolean") {
      return { type: "boolean" };
    }
    if (type.kind === "Number") {
      return { type: "integer" };
    }
    this.unsupported(type, context);
    return { type: "string" };
  }

  namedSchemaRef(type: Type, context: string): SchemaRef {
    if (type.kind === "Model" && isNamedUserModel(type)) {
      this.schemas.set(type.name, type);
      return { ref: type.name };
    }
    if (type.kind === "Enum" && type.name !== "") {
      this.enums.set(type.name, type);
      return { ref: type.name };
    }
    this.report("unnamed-schema", { context }, type);
    return { type: "object" };
  }

  unsupportedType(type: Type, context: string) {
    this.unsupported(type, context);
  }

  unsupportedAuth(context: string, reason: string, target: Type | Operation | Namespace) {
    this.report("unsupported-auth", { context, reason }, target);
  }

  unsupportedResponseStatus(response: HttpOperationResponse) {
    this.report(
      "unsupported-response-status",
      { status: JSON.stringify(response.statusCodes), operation: response.type.kind },
      response.type,
    );
  }

  reservedExtension(key: string, target: Operation) {
    this.report("reserved-extension", { key }, target);
  }

  invalidExtensionKey(key: string, target: Operation) {
    this.report("invalid-extension-key", { key }, target);
  }

  invalidExtensionValue(key: string, path: string, target: Operation) {
    this.report("invalid-extension-value", { key, path }, target);
  }

  emitSchemas(): Record<string, Schema> | undefined {
    const output: Record<string, Schema> = {};
    while (true) {
      const nextModel = [...this.schemas.values()].find((model) => !this.emittedSchemas.has(model.name));
      if (nextModel) {
        this.emittedSchemas.add(nextModel.name);
        output[nextModel.name] = this.schema(nextModel);
        continue;
      }
      const nextEnum = [...this.enums.values()].find((type) => !this.emittedEnums.has(type.name));
      if (nextEnum) {
        this.emittedEnums.add(nextEnum.name);
        output[nextEnum.name] = this.enumSchema(nextEnum);
        continue;
      }
      break;
    }
    return Object.keys(output).length > 0 ? output : undefined;
  }

  private schema(model: Model): Schema {
    const schema: Schema = {
      type: "object",
    };
    const doc = getDoc(this.program, model);
    if (doc) {
      schema.description = doc;
    }
    const properties = [...walkPropertiesInherited(model)];
    if (properties.length > 0) {
      schema.properties = {};
      schema.property_order = [];
      schema.required = [];
      for (const property of properties) {
        schema.properties[property.name] = this.schemaProperty(property);
        schema.property_order.push(property.name);
        if (!property.optional) {
          schema.required.push(property.name);
        }
      }
      if (schema.required.length === 0) {
        delete schema.required;
      }
    }
    return schema;
  }

  private enumSchema(type: Enum): Schema {
    const schema: Schema = {
      type: "string",
      enum: enumValues(type),
    };
    const doc = getDoc(this.program, type);
    if (doc) {
      schema.description = doc;
    }
    return schema;
  }

  private schemaProperty(property: ModelProperty): SchemaProperty {
    const schemaProperty: SchemaProperty = {
      schema: this.schemaRef(property.type, `property ${property.name}`),
    };
    const doc = getDoc(this.program, property);
    if (doc) {
      schemaProperty.description = doc;
    }
    return schemaProperty;
  }

  private inlineObjectRef(model: Model, context: string): SchemaRef {
    if (model.name === "") {
      this.report("unnamed-schema", { context }, model);
    } else {
      this.unsupported(model, context);
    }
    return { type: "object" };
  }

  private unsupported(type: Type, context: string) {
    this.report("unsupported-type", { kind: type.kind, context }, type);
  }

  private report(code: Parameters<typeof reportDiagnostic>[1]["code"], format: any, target: Type) {
    this.failed = true;
    reportDiagnostic(this.program, {
      code,
      format,
      target,
    } as any);
  }
}

export async function $onEmit(context: EmitContext<EmitterOptions>) {
  const outputFile = context.options["output-file"];
  if (!outputFile) {
    reportDiagnostic(context.program, { code: "missing-output-file", target: context.program.getGlobalNamespaceType() });
    return;
  }

  const [services, diagnostics] = getAllHttpServices(context.program);
  context.program.reportDiagnostics(diagnostics);
  if (diagnostics.some((diagnostic) => diagnostic.severity === "error")) {
    return;
  }
  if (services.length !== 1) {
    reportDiagnostic(context.program, {
      code: "multiple-services",
      format: { count: String(services.length) },
      target: context.program.getGlobalNamespaceType(),
    });
    return;
  }

  const service = services[0];
  const builder = new IRBuilder(context.program);
  const doc = buildDocument(context.program, builder, service, context.options);
  doc.schemas = builder.emitSchemas();
  if (builder.hasFailed()) {
    return;
  }

  await emitFile(context.program, {
    path: outputFile,
    content: `${JSON.stringify(doc, null, 2)}\n`,
  });
}

function buildDocument(
  program: Program,
  builder: IRBuilder,
  service: HttpService,
  options: EmitterOptions,
): Document {
  const namespace = service.namespace;
  const info = resolveInfo(program, namespace) ?? {};
  const serviceInfo = getService(program, namespace);
  const tags = getTagsMetadata(program, namespace) ?? [];
  const servers = (getServers(program, namespace) ?? []).map((server) => ({
    url: server.url,
    ...(server.description ? { description: server.description } : {}),
  }));
  const authentication = resolveAuthentication(service);
  const defaultSecurity = authRequirements(builder, authentication.defaultAuth, namespace, "service authentication", true);
  const securitySchemes = collectSecuritySchemes(authentication.schemes);
  const endpoints = service.operations.map((operation) =>
    endpoint(program, builder, operation, authentication.operationsAuth.get(operation.operation), defaultSecurity),
  );

  return prune({
    schema_version: "v2",
    api: { base_path: options["base-path"] ?? "/" },
    info: prune({
      title: info.title ?? serviceInfo?.title ?? "API",
      version: info.version ?? "0.1.0",
      description: info.description,
    }),
    openapi: prune({
      version: "3.0.0",
      tag_order: tags.map((tag) => tag.name),
      security: defaultSecurity,
      security_schemes: Object.keys(securitySchemes).length > 0 ? securitySchemes : undefined,
    }),
    servers: servers.length > 0 ? servers : undefined,
    tags: tags.map((tag) => prune({ name: tag.name, description: tag.description })),
    endpoints,
  }) as Document;
}

function endpoint(
  program: Program,
  builder: IRBuilder,
  operation: HttpOperation,
  operationAuth: AuthenticationReference | undefined,
  defaultSecurity: Record<string, string[]>[] | undefined,
): Endpoint {
  const extensions: Record<string, unknown> = {};
  for (const [key, value] of operationVendorExtensions(program, builder, operation.operation)) {
    extensions[key] = value;
  }
  const authz = getAuthz({ program }, operation.operation);
  if (authz !== undefined) {
    extensions["x-authz"] = authz;
  }
  if (isManual({ program }, operation.operation)) {
    extensions["x-apigen-manual"] = true;
  }

  const output = prune({
    method: operation.verb,
    path: operation.path,
    operation_id: getOperationId(program, operation.operation) ?? resolveOperationId(program, operation.operation),
    summary: getSummary(program, operation.operation),
    description: getDoc(program, operation.operation),
    tags: getAllTags(program, operation.operation),
    parameters: operation.parameters.parameters.map((parameter) => endpointParameter(program, builder, parameter)),
    request_body: requestBody(builder, operation.parameters.body),
    responses: endpointResponses(program, builder, operation.responses),
    cli: cliMetadata(program, operation),
    security: operationSecurity(builder, operation, operationAuth, defaultSecurity),
  }) as Endpoint;
  if (Object.keys(extensions).length > 0) {
    output.extensions = extensions;
  }
  return output;
}

function operationVendorExtensions(
  program: Program,
  builder: IRBuilder,
  operation: Operation,
): [string, unknown][] {
  const extensions = new Map<string, unknown>(getExtensions(program, operation).entries());
  for (const [key, value] of operationExtensionDecoratorLiterals(operation)) {
    extensions.set(key, value);
  }

  const entries = [...extensions.entries()].sort(([left], [right]) => left.localeCompare(right));
  const output: [string, unknown][] = [];
  for (const [key, value] of entries) {
    if (!key.startsWith("x-")) {
      builder.invalidExtensionKey(key, operation);
      continue;
    }
    if (isReservedExtensionKey(key)) {
      builder.reservedExtension(key, operation);
      continue;
    }
    if (!isJSONCompatible(value)) {
      builder.invalidExtensionValue(key, key, operation);
      continue;
    }
    output.push([key, value]);
  }
  return output;
}

function operationExtensionDecoratorLiterals(operation: Operation): [string, unknown][] {
  const decorators = ((operation.node as unknown) as { decorators?: readonly DecoratorNodeLike[] }).decorators ?? [];
  const output: [string, unknown][] = [];
  for (const decorator of decorators) {
    if (decoratorName(decorator.target) !== "extension") {
      continue;
    }
    const args = decorator.arguments ?? [];
    const key = args[0]?.value;
    if (typeof key !== "string" || args[1] === undefined) {
      continue;
    }
    const value = extensionLiteralValue(args[1]);
    if (value.ok) {
      output.push([key, value.value]);
    }
  }
  return output;
}

function decoratorName(target: DecoratorNodeLike["target"]): string | undefined {
  if (target !== undefined && "sv" in target && typeof target.sv === "string") {
    return target.sv;
  }
  if (target !== undefined && "id" in target && typeof target.id?.sv === "string") {
    return target.id.sv;
  }
  return undefined;
}

function extensionLiteralValue(node: ExtensionLiteralNodeLike): LiteralConversionResult {
  if (Array.isArray(node.values)) {
    const values: unknown[] = [];
    for (const item of node.values) {
      const value = extensionLiteralValue(item);
      if (!value.ok) {
        return value;
      }
      values.push(value.value);
    }
    return { ok: true, value: values };
  }

  if (Array.isArray(node.properties)) {
    const output: Record<string, unknown> = {};
    for (const property of node.properties) {
      const key = extensionObjectPropertyName(property);
      if (key === undefined || property.value === undefined) {
        return { ok: false };
      }
      const value = extensionLiteralValue(property.value);
      if (!value.ok) {
        return value;
      }
      output[key] = value.value;
    }
    return { ok: true, value: output };
  }

  switch (typeof node.value) {
    case "string":
    case "boolean":
      return { ok: true, value: node.value };
    case "number":
      return Number.isFinite(node.value) ? { ok: true, value: node.value } : { ok: false };
  }

  if (node.target?.sv === "null" && node.arguments?.length === 0) {
    return { ok: true, value: null };
  }

  return { ok: false };
}

function extensionObjectPropertyName(property: ExtensionObjectPropertyNodeLike): string | undefined {
  if (typeof property.id?.sv === "string") {
    return property.id.sv;
  }
  if (typeof property.id?.value === "string") {
    return property.id.value;
  }
  return undefined;
}

function isReservedExtensionKey(key: string): boolean {
  return key === "x-authz" || key.startsWith("x-apigen-");
}

function isJSONCompatible(value: unknown): boolean {
  if (value === null) {
    return true;
  }
  switch (typeof value) {
    case "string":
    case "boolean":
      return true;
    case "number":
      return Number.isFinite(value);
    case "object":
      if (Array.isArray(value)) {
        return value.every((item) => isJSONCompatible(item));
      }
      return Object.values(value as Record<string, unknown>).every((item) => isJSONCompatible(item));
    default:
      return false;
  }
}

function cliMetadata(program: Program, operation: HttpOperation): unknown {
  const cli = getCLI({ program }, operation.operation);
  if (!cli) {
    return undefined;
  }
  return prune({
    command: cli.command,
    args: cli.args?.map((arg) =>
      prune({
        source: arg.source,
        name: arg.name,
        display_name: arg.displayName,
      }),
    ),
    body_input: cli.bodyInput,
    confirm: cli.confirm,
    output: cli.output
      ? prune({
          mode: cli.output.mode,
          table_columns: cli.output.tableColumns,
          quiet_fields: cli.output.quietFields,
        })
      : undefined,
    pagination: cli.pagination
      ? prune({
          items_field: cli.pagination.itemsField,
          next_page_token_field: cli.pagination.nextPageTokenField,
        })
      : undefined,
  });
}

function endpointParameter(
  program: Program,
  builder: IRBuilder,
  parameter: HttpOperationParameter,
): Parameter {
  const param = parameter.param;
  return prune({
    name: "name" in parameter ? parameter.name : param.name,
    in: parameter.type,
    required: parameter.type === "path" ? true : !param.optional,
    description: getDoc(program, param),
    explode: shouldEmitExplode(builder, param.type, parameter) ? parameter.explode : undefined,
    schema: builder.schemaRef(param.type, `parameter ${param.name}`),
  }) as Parameter;
}

function shouldEmitExplode(
  builder: IRBuilder,
  type: Type,
  parameter: HttpOperationParameter,
): parameter is HttpOperationParameter & { explode: boolean } {
  if (!("explode" in parameter) || parameter.explode === undefined) {
    return false;
  }
  if (parameter.type !== "query" && parameter.type !== "header") {
    return false;
  }
  const schema = builder.schemaRef(type, `parameter ${parameter.param.name}`);
  return schema.type === "array" || parameter.explode === true;
}

function requestBody(builder: IRBuilder, body: HttpPayloadBody | undefined): RequestBody | undefined {
  if (!body) {
    return undefined;
  }
  return prune({
    required: body.property ? !body.property.optional : true,
    contents: bodyContents(builder, body, "request body"),
  }) as RequestBody;
}

function endpointResponses(
  program: Program,
  builder: IRBuilder,
  responses: HttpOperationResponse[],
): Response[] {
  const byStatus = new Map<number, Response>();
  const order: number[] = [];

  for (const httpResponse of responses) {
    const response = endpointResponse(program, builder, httpResponse);
    const existing = byStatus.get(response.status_code);
    if (!existing) {
      byStatus.set(response.status_code, response);
      order.push(response.status_code);
      continue;
    }

    existing.description = existing.description || response.description;
    existing.headers = mergeHeaders(existing.headers, response.headers);
    existing.contents = mergeContents(existing.contents, response.contents);
    existing.extensions = mergeResponseExtensions(existing.extensions, response.extensions);
  }

  return order.map((status) => byStatus.get(status)!);
}

function endpointResponse(
  program: Program,
  builder: IRBuilder,
  response: HttpOperationResponse,
): Response {
  if (typeof response.statusCodes !== "number") {
    builder.unsupportedResponseStatus(response);
  }
  const firstContent = response.responses[0];
  const shape = response.type.kind === "Model" ? getResponseShape({ program }, response.type) : undefined;
  const extensions = shape
    ? {
        "x-apigen-response-shape": prune({
          kind: shape.kind,
          body_type: shape.bodyType,
        }),
      }
    : undefined;

  return prune({
    status_code: typeof response.statusCodes === "number" ? response.statusCodes : 0,
    description: response.description ?? "The request has completed.",
    headers: firstContent ? responseHeaders(program, builder, firstContent) : undefined,
    contents: responseContents(builder, response.responses),
    extensions,
  }) as Response;
}

function mergeHeaders(left: Header[] | undefined, right: Header[] | undefined): Header[] | undefined {
  if (!left || left.length === 0) {
    return right;
  }
  if (!right || right.length === 0) {
    return left;
  }
  const output = [...left];
  const seen = new Set(left.map((header) => header.name.toLowerCase()));
  for (const header of right) {
    const key = header.name.toLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    output.push(header);
  }
  return output;
}

function mergeContents(left: BodyContent[] | undefined, right: BodyContent[] | undefined): BodyContent[] | undefined {
  if (!left || left.length === 0) {
    return right;
  }
  if (!right || right.length === 0) {
    return left;
  }
  const output = [...left];
  for (const content of right) {
    output.push(content);
  }
  return output;
}

function mergeResponseExtensions(
  left: Record<string, unknown> | undefined,
  right: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!left || Object.keys(left).length === 0) {
    return right;
  }
  if (!right || Object.keys(right).length === 0) {
    return left;
  }
  return { ...left, ...right };
}

function responseContents(builder: IRBuilder, contents: HttpOperationResponseContent[]): BodyContent[] | undefined {
  const output: BodyContent[] = [];
  for (const content of contents) {
    if (!content.body) {
      continue;
    }
    output.push(...bodyContents(builder, content.body, "response body"));
  }
  return output.length > 0 ? output : undefined;
}

function bodyContents(builder: IRBuilder, body: HttpPayloadBody, context: string): BodyContent[] {
  switch (body.bodyKind) {
    case "single":
      return body.contentTypes.map((contentType) =>
        prune({
          content_type: contentType,
          body_kind: bodyKindForSingle(body.type, contentType),
          schema: schemaRefForContent(builder, body.type, contentType, context),
        }) as BodyContent,
      );
    case "file":
      return body.contentTypes.map((contentType) =>
        prune({
          content_type: contentType,
          body_kind: "file",
          schema: fileSchemaRef(body.isText),
        }) as BodyContent,
      );
    case "multipart":
      return body.contentTypes.map((contentType) =>
        prune({
          content_type: contentType,
          body_kind: "multipart",
          parts: body.parts.map((part, idx) => multipartPart(builder, part, idx)),
        }) as BodyContent,
      );
  }
}

function multipartPart(builder: IRBuilder, part: HttpOperationPart, idx: number): MultipartPart {
  const bodyKind = part.body.bodyKind === "file" ? "file" : bodyKindForSingle(part.body.type, part.body.contentTypes[0] ?? "application/json");
  const schema =
    part.body.bodyKind === "file"
      ? fileSchemaRef(part.body.isText)
      : schemaRefForContent(builder, part.body.type, part.body.contentTypes[0] ?? "application/json", `multipart part ${part.name ?? idx}`);
  return prune({
    name: part.name ?? `part${idx + 1}`,
    required: !part.optional,
    description: "property" in part && part.property ? getDoc(builder.program, part.property) : undefined,
    content_type: part.body.contentTypes[0],
    body_kind: bodyKind,
    schema,
  }) as MultipartPart;
}

function bodyKindForSingle(type: Type, contentType: string): BodyContent["body_kind"] {
  const normalized = contentType.toLowerCase();
  if (normalized === "application/x-www-form-urlencoded") {
    return "form_urlencoded";
  }
  if (normalized.startsWith("text/")) {
    return "text";
  }
  if (normalized === "application/octet-stream" || isBytesType(type)) {
    return "binary";
  }
  return "json";
}

function schemaRefForContent(builder: IRBuilder, type: Type, contentType: string, context: string): SchemaRef {
  const kind = bodyKindForSingle(type, contentType);
  if ((kind === "binary" || kind === "file") && isBytesType(type)) {
    return { type: "string", format: "binary" };
  }
  return builder.schemaRef(type, context);
}

function fileSchemaRef(isText: boolean): SchemaRef {
  return isText ? { type: "string" } : { type: "string", format: "binary" };
}

function responseHeaders(
  program: Program,
  builder: IRBuilder,
  content: HttpOperationResponseContent,
): Header[] | undefined {
  if (!content.headers) {
    return undefined;
  }
  const headers = Object.entries(content.headers).map(([name, property]) =>
    prune({
      name,
      required: !property.optional,
      description: getDoc(program, property),
      schema: builder.schemaRef(property.type, `response header ${name}`),
    }),
  ) as Header[];
  return headers.length > 0 ? headers : undefined;
}

function collectSecuritySchemes(auths: readonly HttpAuth[]): Record<string, SecurityScheme> {
  const schemes: Record<string, SecurityScheme> = {};
  for (const auth of auths) {
    if (auth.type === "noAuth") {
      continue;
    }
    schemes[auth.id] = securityScheme(auth);
  }
  return schemes;
}

function operationSecurity(
  builder: IRBuilder,
  operation: HttpOperation,
  operationAuth: AuthenticationReference | undefined,
  defaultSecurity: Record<string, string[]>[] | undefined,
): Record<string, string[]>[] | undefined {
  if (!operationAuth) {
    return undefined;
  }
  const security = authRequirements(
    builder,
    operationAuth,
    operation.operation,
    `operation ${operation.operation.name} authentication`,
    defaultSecurity === undefined,
  );
  if (sameSecurity(security, defaultSecurity)) {
    return undefined;
  }
  return security;
}

function authRequirements(
  builder: IRBuilder,
  auth: AuthenticationReference,
  target: Type | Operation | Namespace,
  context: string,
  allowNoAuth: boolean,
): Record<string, string[]>[] | undefined {
  const requirements: Record<string, string[]>[] = [];
  for (const option of auth.options) {
    const requirement: Record<string, string[]> = {};
    for (const ref of option.all) {
      if (ref.kind === "noAuth") {
        if (allowNoAuth) {
          continue;
        }
        builder.unsupportedAuth(
          context,
          "APIGen IR v1 does not support NoAuth operation overrides for secured services.",
          target,
        );
        continue;
      }
      requirement[ref.auth.id] = authScopes(ref);
    }
    if (Object.keys(requirement).length > 0) {
      requirements.push(requirement);
    }
  }
  return requirements.length > 0 ? requirements : undefined;
}

function authScopes(ref: HttpAuthRef): string[] {
  if (ref.kind === "oauth2") {
    return [...ref.scopes];
  }
  return [];
}

function sameSecurity(
  left: Record<string, string[]>[] | undefined,
  right: Record<string, string[]>[] | undefined,
): boolean {
  return JSON.stringify(left ?? []) === JSON.stringify(right ?? []);
}

function securityScheme(scheme: HttpAuth): SecurityScheme {
  switch (scheme.type) {
    case "http":
      return { type: "http", scheme: scheme.scheme };
    case "apiKey":
      return { type: "apiKey", in: scheme.in, name: scheme.name };
    default:
      return { type: scheme.type };
  }
}

function scalarSchemaRef(scalar: Scalar): SchemaRef {
  for (let current: Scalar | undefined = scalar; current; current = current.baseScalar) {
    switch (current.name) {
      case "string":
        return { type: "string" };
      case "boolean":
        return { type: "boolean" };
      case "int8":
      case "int16":
      case "int32":
        return { type: "integer", format: "int32" };
      case "integer":
      case "safeint":
      case "int64":
        return { type: "integer", format: "int64" };
      case "float32":
        return { type: "number", format: "float" };
      case "float64":
      case "decimal":
        return { type: "number", format: "double" };
      case "utcDateTime":
      case "offsetDateTime":
        return { type: "string", format: "date-time" };
      case "plainDate":
        return { type: "string", format: "date" };
      case "bytes":
        return { type: "string", format: "byte" };
    }
  }
  return { type: "string" };
}

function isBytesType(type: Type): boolean {
  if (type.kind !== "Scalar") {
    return false;
  }
  for (let current: Scalar | undefined = type; current; current = current.baseScalar) {
    if (current.name === "bytes") {
      return true;
    }
  }
  return false;
}

function enumValues(type: Enum): string[] {
  return [...type.members.values()].map((member) => String(member.value ?? member.name));
}

function stringLiteralUnionValues(type: Union): string[] | undefined {
  const values: string[] = [];
  for (const variant of type.variants.values()) {
    if (variant.type.kind !== "String") {
      return undefined;
    }
    values.push(variant.type.value);
  }
  return values;
}

function isNamedUserModel(type: Model): boolean {
  return type.name !== "" && !isArrayModelType(type) && !isRecordModelType(type);
}

function prune<T>(value: T): T {
  if (Array.isArray(value)) {
    return value.filter((item) => item !== undefined).map((item) => prune(item)) as T;
  }
  if (value && typeof value === "object") {
    if (isSecurityRequirementObject(value)) {
      return value;
    }
    const output: Record<string, unknown> = {};
    for (const [key, child] of Object.entries(value)) {
      if (child === undefined) {
        continue;
      }
      if (Array.isArray(child) && child.length === 0) {
        continue;
      }
      if (key === "extensions") {
        output[key] = child;
        continue;
      }
      output[key] = prune(child);
    }
    return output as T;
  }
  return value;
}

function isSecurityRequirementObject(value: object): value is Record<string, string[]> {
  const entries = Object.entries(value);
  return entries.length > 0 && entries.every(([, child]) => Array.isArray(child));
}
