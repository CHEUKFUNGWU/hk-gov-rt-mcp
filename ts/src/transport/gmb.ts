import { fetchJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { pick, type Lang } from "../lang.js";

const BASE = "https://data.etagmb.gov.hk";

type Row = Record<string, unknown>;

async function gmb(path: string, ttl: number): Promise<Row> {
  return cached(`gmb:${path}`, ttl, () => fetchJson<Row>(`${BASE}${path}`));
}

export const GMB_REGIONS = ["HKI", "KLN", "NT"] as const;

export async function routeList(region: string | undefined, route: string | undefined): Promise<string> {
  const all = await gmb("/route", TTL.staticRoutes);
  const routes = ((all.data as Row | undefined)?.routes as Record<string, string[]> | undefined) ?? {};
  const q = route?.trim().toLowerCase();
  const lines: string[] = [];
  for (const [reg, arr] of Object.entries(routes)) {
    if (region && reg !== region.toUpperCase()) continue;
    const filtered = q ? arr.filter((r) => r.toLowerCase().startsWith(q)) : arr;
    if (filtered.length === 0) continue;
    lines.push(`${reg}${q ? ` matching "${route}"` : ""} (${filtered.length}): ${filtered.join(", ")}`);
  }
  return lines.length > 0
    ? lines.join("\n")
    : `No GMB routes found${route ? ` for "${route}"` : ""}. Regions: ${GMB_REGIONS.join(", ")}.`;
}

interface GmbVariant extends Row {
  route_id: number;
  route_code: string;
  region: string;
  directions: Row[];
}

export async function routeVariants(region: string, routeCode: string, lang: Lang): Promise<string> {
  const d = await gmb(`/route/${region}/${routeCode}`, TTL.staticRoutes);
  const rows = (d.data as GmbVariant[] | undefined) ?? [];
  if (rows.length === 0) return `No GMB route ${routeCode} in region ${region}. Regions: ${GMB_REGIONS.join(", ")}.`;
  const lines = [`GMB ${region} route ${routeCode} variants:`];
  for (const v of rows) {
    lines.push(`- route_id ${v.route_id} [${pick(v, "description", lang)}]`);
    for (const dir of v.directions ?? []) {
      lines.push(
        `    route_seq ${dir.route_seq}: ${pick(dir, "orig", lang)} → ${pick(dir, "dest", lang)}${dir.remarks_tc || dir.remarks_en ? ` (${pick(dir, "remarks", lang)})` : ""}`,
      );
    }
  }
  return lines.join("\n");
}

export async function routeStops(routeId: string, routeSeq: string, lang: Lang): Promise<string> {
  const d = await gmb(`/route-stop/${routeId}/${routeSeq}`, TTL.staticRoutes);
  const stops = ((d.data as Row | undefined)?.route_stops as Row[] | undefined) ?? [];
  if (stops.length === 0) return `No stops for GMB route_id ${routeId} route_seq ${routeSeq}.`;
  const lines = [`GMB route_id ${routeId} (route_seq ${routeSeq}) — ${stops.length} stops:`];
  for (const s of stops) {
    lines.push(`${s.stop_seq}. ${pick(s, "name", lang)} (stop_id ${s.stop_id})`);
  }
  return lines.join("\n");
}

export async function eta(routeId: string, stopId: string, lang: Lang): Promise<string> {
  const d = await gmb(`/eta/route-stop/${routeId}/${stopId}`, TTL.eta);
  const rows = (d.data as Row[] | undefined) ?? [];
  const active = rows.filter((r) => r.enabled !== false);
  const etas = active.flatMap((r) => (r.eta as Row[] | undefined) ?? []);
  if (etas.length === 0) {
    return `No GMB ETA data for route_id ${routeId} stop_id ${stopId}. Get valid ids via get_gmb_route_list → get_gmb_route_stops.`;
  }
  const lines = [`GMB stop ${stopId} on route_id ${routeId} ETAs:`];
  for (const e of etas) {
    const t = e.timestamp ? String(e.timestamp).slice(11, 16) : "-";
    const diff = e.diff !== undefined ? `${e.diff} min` : "";
    const remark = pick(e, "remark", lang);
    lines.push(`- ${t}${diff ? ` (${diff})` : ""}${remark ? ` ${remark}` : ""}`);
  }
  return lines.join("\n");
}
