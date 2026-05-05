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
export function normalizeURLParams(value) {
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
export function toQueryString(value) {
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
export function toURL(path, value) {
    const query = toQueryString(value);
    return query ? `${path}?${query}` : path;
}
export function toggleArrayValue(value, key, item, checked) {
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
export function clear(value, keys) {
    const next = normalizeURLParams(value);
    const targetKeys = Array.isArray(keys) && keys.length > 0 ? keys : Object.keys(next);
    for (const key of targetKeys) {
        next[key] = Array.isArray(next[key]) ? [] : "";
    }
    return next;
}
export const duckUIURLParams = {
    clear,
    normalize: normalizeURLParams,
    toQueryString,
    toURL,
    toggleArrayValue,
};
