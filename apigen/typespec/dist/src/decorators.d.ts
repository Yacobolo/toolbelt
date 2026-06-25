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
export declare function $cli(context: DecoratorContext, target: Operation, options: CLIOptions): void;
export declare function $authz(context: DecoratorContext, target: Operation, value: unknown): void;
export declare function $manual(context: DecoratorContext, target: Operation): void;
export declare function $responseShape(context: DecoratorContext, target: Model, options: ResponseShapeOptions): void;
export declare const $decorators: {
    apigen: {
        cli: typeof $cli;
        authz: typeof $authz;
        manual: typeof $manual;
        responseShape: typeof $responseShape;
    };
};
export declare function getCLI(context: {
    program: DecoratorContext["program"];
}, target: Operation): CLIOptions | undefined;
export declare function getAuthz(context: {
    program: DecoratorContext["program"];
}, target: Operation): any;
export declare function isManual(context: {
    program: DecoratorContext["program"];
}, target: Operation): boolean;
export declare function getResponseShape(context: {
    program: DecoratorContext["program"];
}, target: Model): ResponseShapeOptions | undefined;
