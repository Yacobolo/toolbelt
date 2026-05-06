type URLParamValue = string | string[];
type URLParamsShape = Record<string, URLParamValue>;

const DATASTAR_URL_SYNC_EVENT = "datastar-url-params-sync";

type URLSyncDetail = {
  params: URLParamsShape;
  url: string;
};

function normalizeStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const seen = new Set<string>();
  const out: string[] = [];
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

function normalizeURLParams(value: unknown): URLParamsShape {
  const record = typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
  const out: URLParamsShape = {};

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

function toQueryString(value: unknown): string {
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

function toURL(path: string, value: unknown): string {
  const query = toQueryString(value);
  return query ? `${path}?${query}` : path;
}

function toggleArrayValue(value: unknown, key: string, item: string, checked: boolean): URLParamsShape {
  const next = normalizeURLParams(value);
  const current = Array.isArray(next[key]) ? [...(next[key] as string[])] : [];
  const trimmed = item.trim();

  if (!trimmed) {
    return next;
  }

  next[key] = checked
    ? Array.from(new Set([...current, trimmed]))
    : current.filter((candidate) => candidate !== trimmed);

  return next;
}

function toggleInput(value: unknown, input: EventTarget | null): URLParamsShape {
  if (!(input instanceof HTMLInputElement)) {
    return normalizeURLParams(value);
  }

  return toggleArrayValue(value, input.name, input.value, input.checked);
}

function clear(value: unknown, keys?: string[]): URLParamsShape {
  const next = normalizeURLParams(value);
  const targetKeys = Array.isArray(keys) && keys.length > 0 ? keys : Object.keys(next);

  for (const key of targetKeys) {
    next[key] = Array.isArray(next[key]) ? [] : "";
  }

  return next;
}

function readLocation(shape: unknown): URLParamsShape {
  const base = normalizeURLParams(shape);
  const url = new URL(window.location.href);
  const next: URLParamsShape = {};

  for (const [key, raw] of Object.entries(base)) {
    if (Array.isArray(raw)) {
      next[key] = url.searchParams.getAll(key).map((item) => item.trim()).filter(Boolean);
      continue;
    }

    next[key] = url.searchParams.get(key)?.trim() ?? raw;
  }

  return next;
}

function emit(params: URLParamsShape): URLParamsShape {
  const detail: URLSyncDetail = {
    params,
    url: `${window.location.pathname}${window.location.search}`,
  };

  window.dispatchEvent(new CustomEvent<URLSyncDetail>(DATASTAR_URL_SYNC_EVENT, { detail }));
  return params;
}

function emitFromLocation(fallback: unknown): URLParamsShape {
  return emit(readLocation(fallback));
}

function replace(value: unknown, path = window.location.pathname): string {
  const next = toURL(path, value);
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.replaceState({}, "", next);
  }
  return next;
}

function push(value: unknown, path = window.location.pathname): string {
  const next = toURL(path, value);
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.pushState({}, "", next);
  }
  return next;
}

let popstateBound = false;

function bindPopstate(fallback: unknown): void {
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

declare global {
  interface Window {
    DatastarURLSync?: typeof datastarURLSync;
    DuckUIURLParams?: typeof duckUIURLParams;
  }
}

window.DuckUIURLParams = duckUIURLParams;
window.DatastarURLSync = datastarURLSync;
