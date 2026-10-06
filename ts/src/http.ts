/** Shared HTTP client for upstream government open-data APIs. */

export class UpstreamError extends Error {
  constructor(
    readonly url: string,
    readonly status: number,
    readonly detail: string,
  ) {
    super(`Upstream ${status} from ${hostOf(url)}: ${detail}`);
    this.name = "UpstreamError";
  }
}

export class UpstreamUnavailableError extends Error {
  constructor(
    readonly url: string,
    readonly detail: string,
  ) {
    super(`Upstream unavailable (${hostOf(url)}): ${detail}`);
    this.name = "UpstreamUnavailableError";
  }
}

function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

const UA = "hk-gov-rt-mcp/1.0 (+https://data.gov.hk open data consumer)";

async function doFetch(url: string, timeoutMs: number, init?: RequestInit): Promise<Response> {
  const res = await fetch(url, {
    ...init,
    headers: {
      "User-Agent": UA,
      Accept: "application/json, text/xml, text/plain;q=0.8",
      ...(init?.headers ?? {}),
    },
    signal: AbortSignal.timeout(timeoutMs),
  });
  if (!res.ok) {
    let body = "";
    try {
      body = (await res.text()).slice(0, 200);
    } catch {
      /* ignore */
    }
    throw new UpstreamError(url, res.status, body || res.statusText);
  }
  return res;
}

export async function fetchJson<T>(url: string, timeoutMs = 10_000, init?: RequestInit): Promise<T> {
  const res = await doFetch(url, timeoutMs, init);
  try {
    return (await res.json()) as T;
  } catch (e) {
    throw new UpstreamUnavailableError(url, `invalid JSON response: ${String(e)}`);
  }
}

export async function fetchText(url: string, timeoutMs = 10_000, init?: RequestInit): Promise<string> {
  const res = await doFetch(url, timeoutMs, init);
  return res.text();
}

export async function postJson<T>(url: string, body: unknown, timeoutMs = 10_000): Promise<T> {
  return fetchJson<T>(url, timeoutMs, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}
