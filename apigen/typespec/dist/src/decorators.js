const cliKey = Symbol.for("@yacobolo/apigen.cli");
const authzKey = Symbol.for("@yacobolo/apigen.authz");
const manualKey = Symbol.for("@yacobolo/apigen.manual");
const responseShapeKey = Symbol.for("@yacobolo/apigen.responseShape");
const packageKey = Symbol.for("@yacobolo/apigen.package");
const contractKey = Symbol.for("@yacobolo/apigen.contract");
const metadataKey = Symbol.for("@yacobolo/apigen.metadata");
const toolKey = Symbol.for("@yacobolo/apigen.tool");
export function $cli(context, target, options) {
    context.program.stateMap(cliKey).set(target, options);
}
export function $authz(context, target, value) {
    context.program.stateMap(authzKey).set(target, value);
}
export function $manual(context, target) {
    context.program.stateSet(manualKey).add(target);
}
export function $responseShape(context, target, options) {
    context.program.stateMap(responseShapeKey).set(target, options);
}
export function $package(context, target, options) {
    context.program.stateMap(packageKey).set(target, options);
}
export function $contract(context, target, options = {}) {
    context.program.stateMap(contractKey).set(target, options);
}
export function $metadata(context, target, value) {
    context.program.stateMap(metadataKey).set(target, value);
}
export function $tool(context, target, options) {
    context.program.stateMap(toolKey).set(target, options);
}
export const $decorators = {
    apigen: {
        cli: $cli,
        authz: $authz,
        manual: $manual,
        responseShape: $responseShape,
        package: $package,
        contract: $contract,
        metadata: $metadata,
        tool: $tool,
    },
};
export function getCLI(context, target) {
    return context.program.stateMap(cliKey).get(target);
}
export function getAuthz(context, target) {
    return context.program.stateMap(authzKey).get(target);
}
export function isManual(context, target) {
    return context.program.stateSet(manualKey).has(target);
}
export function getResponseShape(context, target) {
    return context.program.stateMap(responseShapeKey).get(target);
}
export function getPackages(context) {
    return [...context.program.stateMap(packageKey).entries()];
}
export function getContracts(context) {
    return [...context.program.stateMap(contractKey).entries()];
}
export function getMetadata(context, target) {
    return context.program.stateMap(metadataKey).get(target);
}
export function getTool(context, target) {
    return context.program.stateMap(toolKey).get(target);
}
