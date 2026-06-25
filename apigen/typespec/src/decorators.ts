import type { DecoratorContext, Model, Operation } from "@typespec/compiler";

export interface CLIArg {
  source: "path" | "query" | "body";
  name: string;
  displayName?: string;
}

export interface CLIOutput {
  mode?: "detail" | "collection" | "empty" | "raw";
  tableColumns?: string[];
  quietFields?: string[];
}

export interface CLIPagination {
  itemsField?: string;
  nextPageTokenField?: string;
}

export interface CLIOptions {
  command: string[];
  args?: CLIArg[];
  bodyInput?: "none" | "json" | "flags" | "flags_or_json" | "text" | "binary" | "file" | "multipart";
  confirm?: "none" | "always";
  output?: CLIOutput;
  pagination?: CLIPagination;
}

export interface ResponseShapeOptions {
  kind: "wrapped_json";
  bodyType?: string;
}

const cliKey = Symbol.for("@yacobolo/apigen.cli");
const authzKey = Symbol.for("@yacobolo/apigen.authz");
const manualKey = Symbol.for("@yacobolo/apigen.manual");
const responseShapeKey = Symbol.for("@yacobolo/apigen.responseShape");

export function $cli(context: DecoratorContext, target: Operation, options: CLIOptions) {
  context.program.stateMap(cliKey).set(target, options);
}

export function $authz(context: DecoratorContext, target: Operation, value: unknown) {
  context.program.stateMap(authzKey).set(target, value);
}

export function $manual(context: DecoratorContext, target: Operation) {
  context.program.stateSet(manualKey).add(target);
}

export function $responseShape(
  context: DecoratorContext,
  target: Model,
  options: ResponseShapeOptions,
) {
  context.program.stateMap(responseShapeKey).set(target, options);
}

export const $decorators = {
  apigen: {
    cli: $cli,
    authz: $authz,
    manual: $manual,
    responseShape: $responseShape,
  },
};

export function getCLI(context: { program: DecoratorContext["program"] }, target: Operation) {
  return context.program.stateMap(cliKey).get(target) as CLIOptions | undefined;
}

export function getAuthz(context: { program: DecoratorContext["program"] }, target: Operation) {
  return context.program.stateMap(authzKey).get(target);
}

export function isManual(context: { program: DecoratorContext["program"] }, target: Operation) {
  return context.program.stateSet(manualKey).has(target);
}

export function getResponseShape(
  context: { program: DecoratorContext["program"] },
  target: Model,
) {
  return context.program.stateMap(responseShapeKey).get(target) as ResponseShapeOptions | undefined;
}
