import { fetchJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { pick, type Lang } from "../lang.js";

const BASE = "https://data.etabus.gov.hk/v1/transport/kmb";

type Row = Record<string, unknown>;

export const BOUND_LABEL: Record<Lang, Record<string, string>> = {
  tc: { O: "去程", I: "回程" },
  sc: { O: "去程", I: "回程" },
  en: { O: "outbound", I: "inbound" },
};

const BOUND_WORD: Record<string, string> = { O: "outbound", I: "inbound" };

async function kmb(path: string, ttl: number): Promise<Row> {
  return cached(`kmb:${path}`, ttl, () => fetchJson<Row>(`${BASE}${path}`));
}

export interface KmbRouteVariant {
  co: string;
  route: string;
  bound: string;
  service_type: string;
  orig: string;
  dest: string;
}

export async function findRouteVariants(route: string, lang: Lang): Promise<KmbRouteVariant[]> {
  const all = await kmb("/route", TTL.staticRoutes);
  const rows = (all.data as Row[] | undefined) ?? [];
  const q = route.trim().toLowerCase();
  return rows
    .filter((r) => String(r.route ?? "").toLowerCase().startsWith(q))
    .map((r) => ({
      co: String(r.co ?? ""),
      route: String(r.route),
      bound: String(r.bound ?? ""),
      service_type: String(r.service_type ?? "1"),
      orig: pick(r, "orig", lang),
      dest: pick(r, "dest", lang),
    }));
}

async function stopName(stopId: string, lang: Lang): Promise<string> {
  const s = await cached(`kmb:stop:${stopId}`, TTL.staticRoutes, () =>
    fetchJson<Row>(`${BASE}/stop/${stopId}`),
  );
  return pick((s.data as Row) ?? {}, "name", lang);
}

export async function routeStops(
  route: string,
  bound: string,
  serviceType: string,
  lang: Lang,
): Promise<string> {
  const rs = await kmb(`/route-stop/${route}/${BOUND_WORD[bound] ?? bound}/${serviceType}`, TTL.staticRoutes);
  const rows = (rs.data as Row[] | undefined) ?? [];
  const names = await Promise.all(rows.map((r) => stopName(String(r.stop), lang)));
  const lines = rows.map((r, i) => `${r.seq}. ${names[i]} (stop ${r.stop})`);
  return `KMB route ${route} ${BOUND_LABEL[lang][bound] ?? bound} service_type ${serviceType} — ${rows.length} stops:\n${lines.join("\n")}`;
}

function fmtEta(etas: Row[], lang: Lang): string {
  if (etas.length === 0) return lang === "en" ? "no ETA" : "暫無到站時間";
  return etas
    .map((e) => {
      const t = e.eta ? String(e.eta).slice(11, 16) : "-";
      const rmk = pick(e, "rmk", lang);
      return `${t} (${rmk})`;
    })
    .join(", ");
}

export async function routeEta(route: string, serviceType: string, lang: Lang): Promise<string> {
  const [eta, variants] = await Promise.all([
    kmb(`/route-eta/${route}/${serviceType}`, TTL.eta),
    findRouteVariants(route, lang),
  ]);
  const rows = (eta.data as Row[] | undefined) ?? [];
  if (rows.length === 0) return `No ETA data for KMB route ${route} (service_type ${serviceType}).`;
  const bounds = [...new Set(rows.map((r) => String(r.dir ?? "")))];
  const stopNames = new Map<string, string>();
  await Promise.all(
    bounds.map(async (b) => {
      const rs = await kmb(`/route-stop/${route}/${BOUND_WORD[b] ?? b}/${serviceType}`, TTL.staticRoutes).catch(() => null);
      for (const r of ((rs?.data as Row[] | undefined) ?? [])) {
        stopNames.set(`${b}:${r.seq}`, String(r.stop));
      }
    }),
  );
  const nameOf = (b: string, seq: string) => {
    const sid = stopNames.get(`${b}:${seq}`);
    return sid ? `${sid}` : `seq ${seq}`;
  };
  // group by dir + seq
  const groups = new Map<string, Row[]>();
  for (const r of rows) {
    const key = `${r.dir}:${r.seq}`;
    const g = groups.get(key) ?? [];
    g.push(r);
    groups.set(key, g);
  }
  const lines: string[] = [`KMB route ${route} ETAs (generated ${String(eta.generated_timestamp ?? "")})`];
  for (const b of bounds) {
    lines.push(`-- ${BOUND_LABEL[lang][b] ?? b} --`);
    const seqs = [...new Set(rows.filter((r) => String(r.dir) === b).map((r) => String(r.seq)))].sort(
      (x, y) => Number(x) - Number(y),
    );
    for (const seq of seqs) {
      const dest = pick(rows.find((r) => String(r.dir) === b && String(r.seq) === seq)!, "dest", lang);
      lines.push(`${seq}. ${nameOf(b, seq)} → ${dest}: ${fmtEta(groups.get(`${b}:${seq}`) ?? [], lang)}`);
    }
  }
  const v = variants[0];
  if (v) lines.push(`(route runs ${v.orig} ↔ ${v.dest})`);
  return lines.join("\n");
}

export async function stopEta(stopId: string, lang: Lang): Promise<string> {
  const [eta, name] = await Promise.all([
    kmb(`/stop-eta/${stopId}`, TTL.eta),
    stopName(stopId, lang).catch(() => stopId),
  ]);
  const rows = ((eta.data as Row[] | undefined) ?? []) as Row[];
  if (rows.length === 0) return `No ETA data for KMB stop ${stopId}.`;
  const lines = [`KMB stop ${name} (${stopId}) ETAs:`];
  for (const r of rows) {
    lines.push(
      `- ${r.route} ${BOUND_LABEL[lang][String(r.dir ?? "")] ?? r.dir} → ${pick(r, "dest", lang)}: ${fmtEta([r], lang)}`,
    );
  }
  return lines.join("\n");
}
