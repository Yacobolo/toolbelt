import { duckUIURLParams, normalizeURLParams, toURL } from "./url-params.js";
export const DATASTAR_URL_SYNC_EVENT = "datastar-url-params-sync";
function relativeURL(path, value) {
    return toURL(path, value);
}
function readLocation(shape) {
    const base = normalizeURLParams(shape);
    const url = new URL(window.location.href);
    const next = {};
    for (const [key, raw] of Object.entries(base)) {
        if (Array.isArray(raw)) {
            next[key] = url.searchParams.getAll(key).map((item) => item.trim()).filter(Boolean);
            continue;
        }
        next[key] = url.searchParams.get(key)?.trim() ?? raw;
    }
    return next;
}
function emit(params) {
    const detail = {
        params,
        url: `${window.location.pathname}${window.location.search}`,
    };
    window.dispatchEvent(new CustomEvent(DATASTAR_URL_SYNC_EVENT, { detail }));
    return params;
}
function emitFromLocation(fallback) {
    return emit(readLocation(fallback));
}
function replace(value, path = window.location.pathname) {
    const next = relativeURL(path, value);
    const current = `${window.location.pathname}${window.location.search}`;
    if (next !== current) {
        window.history.replaceState({}, "", next);
    }
    return next;
}
function push(value, path = window.location.pathname) {
    const next = relativeURL(path, value);
    const current = `${window.location.pathname}${window.location.search}`;
    if (next !== current) {
        window.history.pushState({}, "", next);
    }
    return next;
}
let popstateBound = false;
function bindPopstate(fallback) {
    if (popstateBound) {
        return;
    }
    popstateBound = true;
    window.addEventListener("popstate", () => {
        emitFromLocation(fallback);
    });
}
const urlParamsSync = {
    bindPopstate,
    emitFromLocation,
    eventName: DATASTAR_URL_SYNC_EVENT,
    push,
    replace,
};
export const datastarURLSync = {
    urlParams: urlParamsSync,
};
window.DuckUIURLParams = duckUIURLParams;
window.DatastarURLSync = datastarURLSync;
