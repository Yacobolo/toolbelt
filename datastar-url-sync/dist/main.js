const DATASTAR_URL_SYNC_EVENT = "datastar-url-params-sync";
function normalizeURLParams(value) {
    const record = typeof value === "object" && value !== null ? value : {};
    const out = {};
    for (const [key, raw] of Object.entries(record)) {
        if (Array.isArray(raw)) {
            const seen = new Set();
            out[key] = raw.flatMap((item) => {
                if (typeof item !== "string") {
                    return [];
                }
                const trimmed = item.trim();
                if (!trimmed || seen.has(trimmed)) {
                    return [];
                }
                seen.add(trimmed);
                return [trimmed];
            });
            continue;
        }
        if (typeof raw === "string") {
            out[key] = raw.trim();
            continue;
        }
        out[key] = "";
    }
    return out;
}
function toQueryString(value) {
    const params = normalizeURLParams(value);
    const search = new URLSearchParams();
    for (const [key, raw] of Object.entries(params)) {
        if (Array.isArray(raw)) {
            for (const item of raw) {
                search.append(key, item);
            }
            continue;
        }
        if (raw) {
            search.set(key, raw);
        }
    }
    return search.toString();
}
function toURL(path, value) {
    const query = toQueryString(value);
    return query ? `${path}?${query}` : path;
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
    window.dispatchEvent(new CustomEvent(DATASTAR_URL_SYNC_EVENT, {
        detail: {
            params,
            url: `${window.location.pathname}${window.location.search}`,
        },
    }));
    return params;
}
function updateHistory(method, value, path = window.location.pathname) {
    const next = toURL(path, value);
    const current = `${window.location.pathname}${window.location.search}`;
    if (next !== current) {
        window.history[method]({}, "", next);
    }
    return next;
}
function replace(value, path = window.location.pathname) {
    return updateHistory("replaceState", value, path);
}
function push(value, path = window.location.pathname) {
    return updateHistory("pushState", value, path);
}
let popstateBound = false;
function bindPopstate(fallback) {
    if (popstateBound) {
        return;
    }
    popstateBound = true;
    window.addEventListener("popstate", () => {
        emit(readLocation(fallback));
    });
}
const datastarURLSync = {
    bindPopstate,
    push,
    replace,
};
window.DatastarURLSync = datastarURLSync;
export {};
