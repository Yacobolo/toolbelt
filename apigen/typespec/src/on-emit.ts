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
  type HttpOperationResponse,
  type HttpOperationResponseContent,
  type HttpPayloadBody,
  type HttpService,
} from "@typespec/http";
import { getOperationId, getTagsMetadata, resolveInfo, resolveOperationId } from "@typespec/openapi";
import { getAuthz, getCLI, getResponseShape, isManual } from "./decorators.js";
import { type EmitterOptions, reportDiagnostic } from "./lib.js";

interface Document {
  schema_version: "v1";
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
  content_type?: string;
  schema: SchemaRef;
}

interface Response {
  status_code: number;
  description: string;
  headers?: Header[];
  content_type?: string;
  schema?: SchemaRef;
  extensions?: Record<string, unknown>;
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

  constructor(private readonly program: Program) {}

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
    schema_version: "v1",
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
  const authz = getAuthz({ program }, operation.operation);
  if (authz !== undefined) {
    extensions["x-authz"] = authz;
  }
  if (isManual({ program }, operation.operation)) {
    extensions["x-apigen-manual"] = true;
  }

  return prune({
    method: operation.verb,
    path: operation.path,
    operation_id: getOperationId(program, operation.operation) ?? resolveOperationId(program, operation.operation),
    summary: getSummary(program, operation.operation),
    description: getDoc(program, operation.operation),
    tags: getAllTags(program, operation.operation),
    parameters: operation.parameters.parameters.map((parameter) => endpointParameter(program, builder, parameter)),
    request_body: requestBody(builder, operation.parameters.body),
    responses: operation.responses.map((response) => endpointResponse(program, builder, response)),
    cli: cliMetadata(program, operation),
    security: operationSecurity(builder, operation, operationAuth, defaultSecurity),
    extensions: Object.keys(extensions).length > 0 ? extensions : undefined,
  }) as Endpoint;
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
  if (body.bodyKind !== "single") {
    builder.unsupportedType(body.type, "request body");
    return undefined;
  }
  return prune({
    required: body.property ? !body.property.optional : true,
    content_type: body.contentTypes[0],
    schema: builder.namedSchemaRef(body.type, "request body"),
  }) as RequestBody;
}

function endpointResponse(
  program: Program,
  builder: IRBuilder,
  response: HttpOperationResponse,
): Response {
  if (typeof response.statusCodes !== "number") {
    builder.unsupportedResponseStatus(response);
  }
  const content = response.responses[0];
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
    headers: content ? responseHeaders(program, builder, content) : undefined,
    content_type: content?.body?.contentTypes[0],
    schema: content?.body ? builder.schemaRef(content.body.type, "response body") : undefined,
    extensions,
  }) as Response;
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
