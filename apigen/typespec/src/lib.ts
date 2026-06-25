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
