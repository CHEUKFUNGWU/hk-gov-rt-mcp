import { fetchJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { type Lang } from "../lang.js";
import mtrStations from "./data/mtr-stations.json" with { type: "json" };
import lrtStops from "./data/lrt-stops.json" with { type: "json" };

type Row = Record<string, unknown>;

interface StationTable {
  [line: string]: { [station: string]: { tc: string; en: string } };
}

interface LrtTable {
  [stopId: string]: { tc: string; en: string; lines: string[] };
}

const LINES = mtrStations as unknown as { lines: StationTable };
const LRT = lrtStops as unknown as { stops: LrtTable };

export const lineCodes = Object.keys(LINES.lines).map((c) => c.toLowerCase());

function stationName(line: string, station: string, lang: Lang): string {
  const s = LINES.lines[line.toUpperCase()]?.[station.toUpperCase()];
  if (!s) return station;
  return lang === "en" ? s.en : s.tc;
}

/** Heavy rail real-time schedule. `line` like "twl", `station` like "cen"
 *  (comma-separated stations accepted by the upstream API pass through). */
export async function trainSchedule(line: string, station: string, lang: Lang): Promise<string> {
  const url = `https://rt.data.gov.hk/v1/transport/mtr/getSchedule.php?line=${encodeURIComponent(line)}&sta=${encodeURIComponent(station)}`;
  const d = await cached(`mtr:${line}:${station}:${lang}`, TTL.eta, () => fetchJson<Row>(url));
  if (d.error) {
    return `MTR API error ${JSON.stringify(d.error)} — check line/station codes. Valid lines: ${lineCodes.join(", ")}.`;
  }
  const data = (d.data as Record<string, Row> | undefined) ?? {};
  const lines: string[] = [];
  const lc = line.toUpperCase();
  for (const [key, val] of Object.entries(data)) {
    const sta = key.includes("-") ? key.split("-")[1] : key;
    lines.push(`== ${stationName(lc, sta, lang)} (${key}) ==`);
    for (const dir of ["UP", "DOWN"] as const) {
      const trains = (val[dir] as Row[] | undefined) ?? [];
      if (trains.length === 0) continue;
      lines.push(
        `${dir === "UP" ? "↑" : "↓"} ` +
          trains
            .map((t) => {
              const dest = stationName(lc, String(t.dest ?? ""), lang);
              const ttnt = t.ttnt !== undefined && t.ttnt !== "" ? `${t.ttnt} min` : String(t.time ?? "-");
              return `plat ${t.plat ?? "-"} → ${dest} @ ${String(t.time ?? "-")} (${ttnt})`;
            })
            .join("; "),
      );
    }
  }
  if (d.isdelay === true) lines.push("⚠ MTR reports service delay.");
  lines.push(`(sys_time ${String(d.sys_time ?? "")})`);
  return lines.join("\n") || "No schedule data returned.";
}

/** Light rail real-time schedule by numeric station id (e.g. 100). */
export async function lrtSchedule(stationId: string, lang: Lang): Promise<string> {
  const url = `https://rt.data.gov.hk/v1/transport/mtr/lrt/getSchedule?station_id=${encodeURIComponent(stationId)}`;
  const d = await cached(`lrt:${stationId}:${lang}`, TTL.eta, () => fetchJson<Row>(url));
  const name = LRT.stops[stationId];
  const title = name ? `LRT ${lang === "en" ? name.en : name.tc} (${stationId})` : `LRT stop ${stationId}`;
  const platforms = (d.platform_list as Row[] | undefined) ?? [];
  if (platforms.length === 0) {
    const validHint =
      name === undefined
        ? ` Unknown station id — well-known ids include ${Object.keys(LRT.stops).slice(0, 20).join(", ")}…`
        : "";
    return `${title}: no schedule returned (status ${String(d.status ?? "?")}).${validHint}`;
  }
  const lines = [`${title} — arriving trains:`];
  for (const p of platforms) {
    const routes = (p.route_list as Row[] | undefined) ?? [];
    for (const r of routes) {
      const t = lang === "en" ? r.time_en : r.time_ch;
      const dest = lang === "en" ? r.dest_en : r.dest_ch;
      lines.push(`- plat ${p.platform_id} | route ${r.route_no} → ${dest}: ${t}`);
    }
  }
  return lines.join("\n");
}

export const lrtStopIds = Object.keys(LRT.stops);

export function lrtStopsByLine(): string {
  const byLine = new Map<string, string[]>();
  for (const [id, s] of Object.entries(LRT.stops)) {
    for (const ln of s.lines) {
      byLine.set(ln, [...(byLine.get(ln) ?? []), `${ln} stop ${s.tc}/${s.en} = id ${id}`]);
    }
  }
  return [...byLine.entries()].map(([ln, arr]) => `${ln}: ${arr.join("; ")}`).join("\n");
}

/** MTR Bus real-time schedule (upstream has been returning 404; reported
 *  gracefully so the tool stays usable when the feed returns). */
export async function busSchedule(route: string): Promise<string> {
  const url = `https://rt.data.gov.hk/v1/transport/mtr/bus/getSchedule?route=${encodeURIComponent(route)}`;
  try {
    const d = await cached(`mtrbus:${route}`, TTL.eta, () => fetchJson<Row>(url));
    return `MTR bus route ${route}:\n${JSON.stringify(d, null, 1)}`;
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return `MTR bus schedule is currently unavailable upstream (${msg}). The endpoint is officially listed at ${url} — try again later.`;
  }
}

export async function lookupStationCodes(query: string, lang: Lang): Promise<string> {
  const q = query.trim().toLowerCase();
  const lines: string[] = [];
  for (const [lc, stations] of Object.entries(LINES.lines)) {
    for (const [sc, s] of Object.entries(stations)) {
      const name = lang === "en" ? s.en : s.tc;
      const other = lang === "en" ? s.tc : s.en;
      if (name.toLowerCase().includes(q) || other.toLowerCase().includes(q) || sc.toLowerCase() === q) {
        lines.push(`${lc} line — ${sc} = ${name} (${other})`);
      }
    }
  }
  return lines.length > 0 ? lines.join("\n") : `No station matching "${query}". Valid lines: ${lineCodes.join(", ")}`;
}
