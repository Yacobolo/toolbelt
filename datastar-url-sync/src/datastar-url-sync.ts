import { duckUIURLParams, normalizeURLParams, toURL, type URLParamsShape } from "./url-params.js";

export const DATASTAR_URL_SYNC_EVENT = "datastar-url-params-sync";

type URLSyncDetail = {
  params: URLParamsShape;
  url: string;
};

function relativeURL(path: string, value: unknown): string {
  return toURL(path, value);
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
  const next = relativeURL(path, value);
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.replaceState({}, "", next);
  }
  return next;
}

function push(value: unknown, path = window.location.pathname): string {
  const next = relativeURL(path, value);
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

declare global {
  interface Window {
    DatastarURLSync?: typeof datastarURLSync;
    DuckUIURLParams?: typeof duckUIURLParams;
  }
}

window.DuckUIURLParams = duckUIURLParams;
window.DatastarURLSync = datastarURLSync;
