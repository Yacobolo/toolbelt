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
  bodyInput?: "none" | "json" | "flags" | "flags_or_json";
  confirm?: "none" | "always";
  output?: CLIOutput;
  pagination?: CLIPagination;
}

model ResponseShapeOptions {
  kind: "wrapped_json";
  bodyType?: string;
}

extern dec cli(target: Operation, options: valueof CLIOptions);
extern dec authz(target: Operation, value: valueof unknown);
extern dec manual(target: Operation);
extern dec responseShape(target: Model, options: valueof ResponseShapeOptions);
