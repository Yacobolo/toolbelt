import "@typespec/http";
import "@typespec/openapi";
import "../dist/src/index.js";

using TypeSpec.Reflection;

namespace apigen;

model CLIArg {
  source: "path" | "query" | "body";
  name: string;
  displayName?: string;
}

model CLIOutput {
  mode?: "detail" | "collection" | "empty" | "raw";
  tableColumns?: string[];
  quietFields?: string[];
}

model CLIPagination {
  itemsField?: string;
  nextPageTokenField?: string;
}

model CLIOptions {
  command: string[];
  args?: CLIArg[];
  bodyInput?: "none" | "json" | "flags" | "flags_or_json" | "text" | "binary" | "file" | "multipart";
  confirm?: "none" | "always";
  output?: CLIOutput;
  pagination?: CLIPagination;
}

model ResponseShapeOptions {
  kind: "wrapped_json";
  bodyType?: string;
}

model PackageOptions {
  title: string;
  version: string;
  description?: string;
}

model ContractOptions {
  name?: string;
  kind?: string;
  tags?: string[];
}

union ToolInputMode {
  modelValue: "model";
  contextValue: "context";
  omitValue: "omit";
}

model ToolInputFieldOptions {
  source: "path" | "query" | "header" | "body";
  name: string;
  mode?: ToolInputMode;
  `alias`?: string;
  contextKey?: string;
  description?: string;
  `default`?: unknown;
}

model ToolProjectionOptions {
  source: string;
  target?: string;
  select?: ToolProjectionOptions[];
  countAs?: string;
}

model ToolCursorOptions {
  source: string;
  target?: string;
  hasMoreTarget?: string;
}

model ToolOutputOptions {
  mode: "raw" | "project" | "empty";
  select?: ToolProjectionOptions[];
  cursor?: ToolCursorOptions;
}

model ToolInputOptions {
  fields?: ToolInputFieldOptions[];
}

model ToolOptions {
  name: string;
  description?: string;
  effect: "read" | "idempotent-write" | "write" | "destructive";
  confirmation?: "never" | "policy" | "always";
  tags?: string[];
  input?: ToolInputOptions;
  output: ToolOutputOptions;
  metadata?: unknown;
}

extern dec cli(target: Operation, options: valueof CLIOptions);
extern dec authz(target: Operation, value: valueof unknown);
extern dec manual(target: Operation);
extern dec responseShape(target: Model, options: valueof ResponseShapeOptions);
extern dec `package`(target: Namespace, options: valueof PackageOptions);
extern dec contract(target: Model | Enum, options?: valueof ContractOptions);
extern dec `metadata`(target: Model | ModelProperty, value: valueof unknown);
extern dec tool(target: Operation, options: valueof ToolOptions);
