const cliKey = Symbol.for("@yacobolo/apigen.cli");
const authzKey = Symbol.for("@yacobolo/apigen.authz");
const manualKey = Symbol.for("@yacobolo/apigen.manual");
const responseShapeKey = Symbol.for("@yacobolo/apigen.responseShape");
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
export const $decorators = {
    apigen: {
        cli: $cli,
        authz: $authz,
        manual: $manual,
        responseShape: $responseShape,
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
