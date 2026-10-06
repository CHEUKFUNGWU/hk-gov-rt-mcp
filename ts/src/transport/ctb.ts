import { fetchJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { pick, type Lang } from "../lang.js";
import { BOUND_LABEL } from "./kmb.js";

const BASE = "https://rt.data.gov.hk/v2/transport/citybus";

type Row = Record<string, unknown>;

async function ctb(path: string, ttl: number): Promise<Row> {
  return cached(`ctb:${path}`, ttl, () => fetchJson<Row>(`${BASE}${path}`));
}

export async function findRoutes(route: string, lang: Lang): Promise<Row[]> {
  const all = await cached("ctb:/route/CTB", TTL.staticRoutes, () =>
    fetchJson<Row>(`${BASE}/route/CTB`, 30_000),
  );
  const rows = (all.data as Row[] | undefined) ?? [];
  const q = route.trim().toLowerCase();
  return rows.filter((r) => String(r.route ?? "").toLowerCase().startsWith(q));
}

async function stopName(stopId: string, lang: Lang): Promise<string> {
  const s = await cached(`ctb:stop:${stopId}`, TTL.staticRoutes, () =>
    fetchJson<Row>(`${BASE}/stop/${stopId}`),
  );
  return pick((s.data as Row) ?? {}, "name", lang);
}

export async function routeStops(route: string, direction: string, lang: Lang): Promise<string> {
  const rs = await ctb(`/route-stop/CTB/${route}/${direction}`, TTL.staticRoutes);
  const rows = (rs.data as Row[] | undefined) ?? [];
  const names = await Promise.all(rows.map((r) => stopName(String(r.stop), lang)));
  const lines = rows.map((r, i) => `${r.seq}. ${names[i]} (stop ${r.stop})`);
  return `CTB route ${route} ${direction} — ${rows.length} stops:\n${lines.join("\n")}`;
}

/** Citybus exposes per-stop ETA (/eta/CTB/{stop}/{route}); the route-wide
 *  endpoints (route-eta, stop-eta) currently return 422, so route ETA is
 *  assembled by querying every stop of the direction in parallel. */
async function stopEtas(route: string, stopId: string): Promise<Row[]> {
  const e = await ctb(`/eta/CTB/${stopId}/${route}`, TTL.eta);
  return (e.data as Row[] | undefined) ?? [];
}

export async function routeEta(route: string, direction: string, lang: Lang): Promise<string> {
  const rs = await ctb(`/route-stop/CTB/${route}/${direction}`, TTL.staticRoutes);
  const rows = (rs.data as Row[] | undefined) ?? [];
  if (rows.length === 0) return `No stops found for CTB route ${route} (${direction}).`;
  const names = await Promise.all(rows.map((r) => stopName(String(r.stop), lang)));
  const etas = await Promise.all(rows.map((r) => stopEtas(route, String(r.stop)).catch(() => [] as Row[])));
  const lines = [`CTB route ${route} ${direction} ETAs (stop-by-stop):`];
  let any = false;
  rows.forEach((r, i) => {
    const list = etas[i] ?? [];
    const txt =
      list.length === 0
        ? lang === "en"
          ? "no ETA"
          : "暫無到站時間"
        : list
            .map((e) => {
              const t = e.eta ? String(e.eta).slice(11, 16) : "-";
              return `${t} (${pick(e, "rmk", lang)})`;
            })
            .join(", ");
    if (list.length > 0) any = true;
    lines.push(`${r.seq}. ${names[i]}: ${txt}`);
  });
  if (!any) lines.push(lang === "en" ? "No ETA data available for this route right now." : "現時此路線沒有到站時間資料。");
  return lines.join("\n");
}

export async function stopEta(route: string, stopId: string, lang: Lang): Promise<string> {
  const [etas, name] = await Promise.all([
    stopEtas(route, stopId),
    stopName(stopId, lang).catch(() => stopId),
  ]);
  if (etas.length === 0) return `No ETA data for CTB stop ${name} (${stopId}) on route ${route}.`;
  const dir = String(etas[0]?.dir ?? "");
  const lines = [
    `CTB stop ${name} (${stopId}) route ${route}${dir ? ` ${BOUND_LABEL[lang][dir] ?? dir}` : ""}:`,
    ...etas.map((e) => {
      const t = e.eta ? String(e.eta).slice(11, 16) : "-";
      return `- ${t} → ${pick(e, "dest", lang)} (${pick(e, "rmk", lang)})`;
    }),
  ];
  return lines.join("\n");
}
