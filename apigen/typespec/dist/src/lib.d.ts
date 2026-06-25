export declare const $lib: import("@typespec/compiler").TypeSpecLibrary<{
    "unsupported-type": {
        readonly default: import("@typespec/compiler").CallableMessage<["kind", "context"]>;
    };
    "unsupported-response-status": {
        readonly default: import("@typespec/compiler").CallableMessage<["status", "operation"]>;
    };
    "unsupported-response-content": {
        readonly default: import("@typespec/compiler").CallableMessage<["operation", "status", "contentType"]>;
    };
    "unsupported-inline-enum": {
        readonly default: import("@typespec/compiler").CallableMessage<["context"]>;
    };
    "unsupported-cookie": {
        readonly default: "cookie parameters are not supported by APIGen v0.3.2 generated server, OpenAPI, and CLI outputs.";
    };
    "unsupported-shared-route": {
        readonly default: import("@typespec/compiler").CallableMessage<["reason"]>;
    };
    "unsupported-auth": {
        readonly default: import("@typespec/compiler").CallableMessage<["context", "reason"]>;
    };
    "multiple-services": {
        readonly default: import("@typespec/compiler").CallableMessage<["count"]>;
    };
    "reserved-extension": {
        readonly default: import("@typespec/compiler").CallableMessage<["key"]>;
    };
    "invalid-extension-key": {
        readonly default: import("@typespec/compiler").CallableMessage<["key"]>;
    };
    "invalid-extension-value": {
        readonly default: import("@typespec/compiler").CallableMessage<["key", "path"]>;
    };
    "unnamed-schema": {
        readonly default: import("@typespec/compiler").CallableMessage<["context"]>;
    };
    "missing-output-file": {
        readonly default: "The APIGen emitter option 'output-file' is required.";
    };
}, Record<string, any>, never>;
export declare const reportDiagnostic: <C extends "unsupported-type" | "unsupported-response-status" | "unsupported-response-content" | "unsupported-inline-enum" | "unsupported-cookie" | "unsupported-shared-route" | "unsupported-auth" | "multiple-services" | "reserved-extension" | "invalid-extension-key" | "invalid-extension-value" | "unnamed-schema" | "missing-output-file", M extends keyof {
    "unsupported-type": {
        readonly default: import("@typespec/compiler").CallableMessage<["kind", "context"]>;
    };
    "unsupported-response-status": {
        readonly default: import("@typespec/compiler").CallableMessage<["status", "operation"]>;
    };
    "unsupported-response-content": {
        readonly default: import("@typespec/compiler").CallableMessage<["operation", "status", "contentType"]>;
    };
    "unsupported-inline-enum": {
        readonly default: import("@typespec/compiler").CallableMessage<["context"]>;
    };
    "unsupported-cookie": {
        readonly default: "cookie parameters are not supported by APIGen v0.3.2 generated server, OpenAPI, and CLI outputs.";
    };
    "unsupported-shared-route": {
        readonly default: import("@typespec/compiler").CallableMessage<["reason"]>;
    };
    "unsupported-auth": {
        readonly default: import("@typespec/compiler").CallableMessage<["context", "reason"]>;
    };
    "multiple-services": {
        readonly default: import("@typespec/compiler").CallableMessage<["count"]>;
    };
    "reserved-extension": {
        readonly default: import("@typespec/compiler").CallableMessage<["key"]>;
    };
    "invalid-extension-key": {
        readonly default: import("@typespec/compiler").CallableMessage<["key"]>;
    };
    "invalid-extension-value": {
        readonly default: import("@typespec/compiler").CallableMessage<["key", "path"]>;
    };
    "unnamed-schema": {
        readonly default: import("@typespec/compiler").CallableMessage<["context"]>;
    };
    "missing-output-file": {
        readonly default: "The APIGen emitter option 'output-file' is required.";
    };
}[C]>(program: import("@typespec/compiler").Program, diag: import("@typespec/compiler").DiagnosticReport<{
    "unsupported-type": {
        readonly default: import("@typespec/compiler").CallableMessage<["kind", "context"]>;
    };
    "unsupported-response-status": {
        readonly default: import("@typespec/compiler").CallableMessage<["status", "operation"]>;
    };
    "unsupported-response-content": {
        readonly default: import("@typespec/compiler").CallableMessage<["operation", "status", "contentType"]>;
    };
    "unsupported-inline-enum": {
        readonly default: import("@typespec/compiler").CallableMessage<["context"]>;
    };
    "unsupported-cookie": {
        readonly default: "cookie parameters are not supported by APIGen v0.3.2 generated server, OpenAPI, and CLI outputs.";
    };
    "unsupported-shared-route": {
        readonly default: import("@typespec/compiler").CallableMessage<["reason"]>;
    };
    "unsupported-auth": {
        readonly default: import("@typespec/compiler").CallableMessage<["context", "reason"]>;
    };
    "multiple-services": {
        readonly default: import("@typespec/compiler").CallableMessage<["count"]>;
    };
    "reserved-extension": {
        readonly default: import("@typespec/compiler").CallableMessage<["key"]>;
    };
    "invalid-extension-key": {
        readonly default: import("@typespec/compiler").CallableMessage<["key"]>;
    };
    "invalid-extension-value": {
        readonly default: import("@typespec/compiler").CallableMessage<["key", "path"]>;
    };
    "unnamed-schema": {
        readonly default: import("@typespec/compiler").CallableMessage<["context"]>;
    };
    "missing-output-file": {
        readonly default: "The APIGen emitter option 'output-file' is required.";
    };
}, C, M>) => void;
export interface EmitterOptions {
    "output-file"?: string;
    "base-path"?: string;
}
