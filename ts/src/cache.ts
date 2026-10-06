/** Simple in-memory TTL cache. stdio MCP servers are per-client processes, so
 *  a process-local map is all that is needed to stay within upstream update
 *  cadences. */

interface Entry {
  expires: number;
  value: unknown;
}

const store = new Map<string, Entry>();

export async function cached<T>(key: string, ttlMs: number, load: () => Promise<T>): Promise<T> {
  const hit = store.get(key);
  const now = Date.now();
  if (hit && hit.expires > now) {
    return hit.value as T;
  }
  const value = await load();
  store.set(key, { expires: now + ttlMs, value });
  return value;
}

/** TTL presets shared by both implementations (mirrored in Go). */
export const TTL = {
  eta: 20_000,
  weatherNow: 60_000,
  warnings: 60_000,
  forecast: 10 * 60_000,
  traffic: 2 * 60_000,
  parking: 60_000,
  staticRoutes: 6 * 60 * 60_000,
} as const;
