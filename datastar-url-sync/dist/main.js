const DATASTAR_URL_SYNC_EVENT = "datastar-url-params-sync";
function normalizeStringArray(value) {
    if (!Array.isArray(value)) {
        return [];
    }
    const seen = new Set();
    const out = [];
    for (const item of value) {
        if (typeof item !== "string") {
            continue;
        }
        const trimmed = item.trim();
        if (!trimmed || seen.has(trimmed)) {
            continue;
        }
        seen.add(trimmed);
        out.push(trimmed);
    }
    return out;
}
function normalizeURLParams(value) {
    const record = typeof value === "object" && value !== null ? value : {};
    const out = {};
    for (const [key, raw] of Object.entries(record)) {
        if (Array.isArray(raw)) {
            out[key] = normalizeStringArray(raw);
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
function toggleArrayValue(value, key, item, checked) {
    const next = normalizeURLParams(value);
    const current = Array.isArray(next[key]) ? [...next[key]] : [];
    const trimmed = item.trim();
    if (!trimmed) {
        return next;
    }
    next[key] = checked
        ? Array.from(new Set([...current, trimmed]))
        : current.filter((candidate) => candidate !== trimmed);
    return next;
}
function toggleInput(value, input) {
    if (!(input instanceof HTMLInputElement)) {
        return normalizeURLParams(value);
    }
    return toggleArrayValue(value, input.name, input.value, input.checked);
}
function clear(value, keys) {
    const next = normalizeURLParams(value);
    const targetKeys = Array.isArray(keys) && keys.length > 0 ? keys : Object.keys(next);
    for (const key of targetKeys) {
        next[key] = Array.isArray(next[key]) ? [] : "";
    }
    return next;
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
    const next = toURL(path, value);
    const current = `${window.location.pathname}${window.location.search}`;
    if (next !== current) {
        window.history.replaceState({}, "", next);
    }
    return next;
}
function push(value, path = window.location.pathname) {
    const next = toURL(path, value);
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
const duckUIURLParams = {
    clear,
    normalize: normalizeURLParams,
    toQueryString,
    toURL,
    toggleArrayValue,
    toggleInput,
};
const datastarURLSync = {
    bindPopstate,
    eventName: DATASTAR_URL_SYNC_EVENT,
    push,
    replace,
};
window.DuckUIURLParams = duckUIURLParams;
window.DatastarURLSync = datastarURLSync;
export {};
