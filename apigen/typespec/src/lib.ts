import { createTypeSpecLibrary, paramMessage } from "@typespec/compiler";

export const $lib = createTypeSpecLibrary({
  name: "@yacobolo/apigen",
  diagnostics: {
    "unsupported-type": {
      severity: "error",
      messages: {
        default: paramMessage`Unsupported TypeSpec type '${"kind"}' in ${"context"}.`,
      },
    },
    "unsupported-response-status": {
      severity: "error",
      messages: {
        default: paramMessage`Unsupported response status '${"status"}' for operation '${"operation"}'. APIGen IR v1 requires concrete numeric status codes.`,
      },
    },
    "unsupported-inline-enum": {
      severity: "error",
      messages: {
        default: paramMessage`Unsupported inline enum-like union in ${"context"}. APIGen IR v1 requires a named enum schema.`,
      },
    },
    "unsupported-auth": {
      severity: "error",
      messages: {
        default: paramMessage`Unsupported authentication shape in ${"context"}. ${"reason"}`,
      },
    },
    "multiple-services": {
      severity: "error",
      messages: {
        default: paramMessage`APIGen TypeSpec emitter requires exactly one TypeSpec service, found ${"count"}.`,
      },
    },
    "reserved-extension": {
      severity: "error",
      messages: {
        default: paramMessage`Operation extension '${"key"}' is reserved for APIGen-owned metadata. Use the matching APIGen decorator instead.`,
      },
    },
    "invalid-extension-key": {
      severity: "error",
      messages: {
        default: paramMessage`Operation extension '${"key"}' must start with 'x-'.`,
      },
    },
    "invalid-extension-value": {
      severity: "error",
      messages: {
        default: paramMessage`Operation extension '${"key"}' contains a non-JSON-compatible value at ${"path"}.`,
      },
    },
    "unnamed-schema": {
      severity: "error",
      messages: {
        default: paramMessage`APIGen IR v1 requires ${"context"} to resolve to a named model schema.`,
      },
    },
    "missing-output-file": {
      severity: "error",
      messages: {
        default: "The APIGen emitter option 'output-file' is required.",
      },
    },
  },
  emitter: {
    options: {
      type: "object",
      additionalProperties: false,
      properties: {
        "output-file": {
          type: "string",
          format: "absolute-path",
          nullable: true,
        },
        "base-path": {
          type: "string",
          nullable: true,
        },
      },
      required: [],
    },
  },
} as const);

export const { reportDiagnostic } = $lib;

export interface EmitterOptions {
  "output-file"?: string;
  "base-path"?: string;
}
