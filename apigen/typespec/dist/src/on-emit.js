import { emitFile, getAllTags, getDoc, getService, getSummary, isArrayModelType, isRecordModelType, walkPropertiesInherited, } from "@typespec/compiler";
import { getAllHttpServices, getServers, resolveAuthentication, } from "@typespec/http";
import { getExtensions, getOperationId, getTagsMetadata, resolveInfo, resolveOperationId } from "@typespec/openapi";
import { getAuthz, getCLI, getResponseShape, isManual } from "./decorators.js";
import { reportDiagnostic } from "./lib.js";
class IRBuilder {
    program;
    schemas = new Map();
    enums = new Map();
    emittedSchemas = new Set();
    emittedEnums = new Set();
    failed = false;
    constructor(program) {
        this.program = program;
    }
    hasFailed() {
        return this.failed;
    }
    schemaRef(type, context) {
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
    namedSchemaRef(type, context) {
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
    unsupportedType(type, context) {
        this.unsupported(type, context);
    }
    unsupportedAuth(context, reason, target) {
        this.report("unsupported-auth", { context, reason }, target);
    }
    unsupportedResponseStatus(response) {
        this.report("unsupported-response-status", { status: JSON.stringify(response.statusCodes), operation: response.type.kind }, response.type);
    }
    reservedExtension(key, target) {
        this.report("reserved-extension", { key }, target);
    }
    invalidExtensionKey(key, target) {
        this.report("invalid-extension-key", { key }, target);
    }
    invalidExtensionValue(key, path, target) {
        this.report("invalid-extension-value", { key, path }, target);
    }
    emitSchemas() {
        const output = {};
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
    schema(model) {
        const schema = {
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
    enumSchema(type) {
        const schema = {
            type: "string",
            enum: enumValues(type),
        };
        const doc = getDoc(this.program, type);
        if (doc) {
            schema.description = doc;
        }
        return schema;
    }
    schemaProperty(property) {
        const schemaProperty = {
            schema: this.schemaRef(property.type, `property ${property.name}`),
        };
        const doc = getDoc(this.program, property);
        if (doc) {
            schemaProperty.description = doc;
        }
        return schemaProperty;
    }
    inlineObjectRef(model, context) {
        if (model.name === "") {
            this.report("unnamed-schema", { context }, model);
        }
        else {
            this.unsupported(model, context);
        }
        return { type: "object" };
    }
    unsupported(type, context) {
        this.report("unsupported-type", { kind: type.kind, context }, type);
    }
    report(code, format, target) {
        this.failed = true;
        reportDiagnostic(this.program, {
            code,
            format,
            target,
        });
    }
}
export async function $onEmit(context) {
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
function buildDocument(program, builder, service, options) {
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
    const endpoints = service.operations.map((operation) => endpoint(program, builder, operation, authentication.operationsAuth.get(operation.operation), defaultSecurity));
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
    });
}
function endpoint(program, builder, operation, operationAuth, defaultSecurity) {
    const extensions = {};
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
    });
}
function operationVendorExtensions(program, builder, operation) {
    const entries = [...getExtensions(program, operation).entries()];
    entries.sort(([left], [right]) => left.localeCompare(right));
    const output = [];
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
function isReservedExtensionKey(key) {
    return key === "x-authz" || key.startsWith("x-apigen-");
}
function isJSONCompatible(value) {
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
            return Object.values(value).every((item) => isJSONCompatible(item));
        default:
            return false;
    }
}
function cliMetadata(program, operation) {
    const cli = getCLI({ program }, operation.operation);
    if (!cli) {
        return undefined;
    }
    return prune({
        command: cli.command,
        args: cli.args?.map((arg) => prune({
            source: arg.source,
            name: arg.name,
            display_name: arg.displayName,
        })),
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
function endpointParameter(program, builder, parameter) {
    const param = parameter.param;
    return prune({
        name: "name" in parameter ? parameter.name : param.name,
        in: parameter.type,
        required: parameter.type === "path" ? true : !param.optional,
        description: getDoc(program, param),
        explode: shouldEmitExplode(builder, param.type, parameter) ? parameter.explode : undefined,
        schema: builder.schemaRef(param.type, `parameter ${param.name}`),
    });
}
function shouldEmitExplode(builder, type, parameter) {
    if (!("explode" in parameter) || parameter.explode === undefined) {
        return false;
    }
    if (parameter.type !== "query" && parameter.type !== "header") {
        return false;
    }
    const schema = builder.schemaRef(type, `parameter ${parameter.param.name}`);
    return schema.type === "array" || parameter.explode === true;
}
function requestBody(builder, body) {
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
    });
}
function endpointResponse(program, builder, response) {
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
    });
}
function responseHeaders(program, builder, content) {
    if (!content.headers) {
        return undefined;
    }
    const headers = Object.entries(content.headers).map(([name, property]) => prune({
        name,
        required: !property.optional,
        description: getDoc(program, property),
        schema: builder.schemaRef(property.type, `response header ${name}`),
    }));
    return headers.length > 0 ? headers : undefined;
}
function collectSecuritySchemes(auths) {
    const schemes = {};
    for (const auth of auths) {
        if (auth.type === "noAuth") {
            continue;
        }
        schemes[auth.id] = securityScheme(auth);
    }
    return schemes;
}
function operationSecurity(builder, operation, operationAuth, defaultSecurity) {
    if (!operationAuth) {
        return undefined;
    }
    const security = authRequirements(builder, operationAuth, operation.operation, `operation ${operation.operation.name} authentication`, defaultSecurity === undefined);
    if (sameSecurity(security, defaultSecurity)) {
        return undefined;
    }
    return security;
}
function authRequirements(builder, auth, target, context, allowNoAuth) {
    const requirements = [];
    for (const option of auth.options) {
        const requirement = {};
        for (const ref of option.all) {
            if (ref.kind === "noAuth") {
                if (allowNoAuth) {
                    continue;
                }
                builder.unsupportedAuth(context, "APIGen IR v1 does not support NoAuth operation overrides for secured services.", target);
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
function authScopes(ref) {
    if (ref.kind === "oauth2") {
        return [...ref.scopes];
    }
    return [];
}
function sameSecurity(left, right) {
    return JSON.stringify(left ?? []) === JSON.stringify(right ?? []);
}
function securityScheme(scheme) {
    switch (scheme.type) {
        case "http":
            return { type: "http", scheme: scheme.scheme };
        case "apiKey":
            return { type: "apiKey", in: scheme.in, name: scheme.name };
        default:
            return { type: scheme.type };
    }
}
function scalarSchemaRef(scalar) {
    for (let current = scalar; current; current = current.baseScalar) {
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
function enumValues(type) {
    return [...type.members.values()].map((member) => String(member.value ?? member.name));
}
function stringLiteralUnionValues(type) {
    const values = [];
    for (const variant of type.variants.values()) {
        if (variant.type.kind !== "String") {
            return undefined;
        }
        values.push(variant.type.value);
    }
    return values;
}
function isNamedUserModel(type) {
    return type.name !== "" && !isArrayModelType(type) && !isRecordModelType(type);
}
function prune(value) {
    if (Array.isArray(value)) {
        return value.filter((item) => item !== undefined).map((item) => prune(item));
    }
    if (value && typeof value === "object") {
        if (isSecurityRequirementObject(value)) {
            return value;
        }
        const output = {};
        for (const [key, child] of Object.entries(value)) {
            if (child === undefined) {
                continue;
            }
            if (Array.isArray(child) && child.length === 0) {
                continue;
            }
            output[key] = prune(child);
        }
        return output;
    }
    return value;
}
function isSecurityRequirementObject(value) {
    const entries = Object.entries(value);
    return entries.length > 0 && entries.every(([, child]) => Array.isArray(child));
}
