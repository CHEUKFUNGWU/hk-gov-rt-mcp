import { fetchText, postJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { type Lang } from "../lang.js";

/** Transport Department traffic camera snapshot API (CID). Camera ids follow
 *  the TD dataspec, e.g. A01–A12 (Hong Kong Island), KE-series, NT-series.
 *  The endpoint has been observed to reject some networks (403 from
 *  CloudFront); failures are reported so callers can retry later. */
export async function cameraSnapshots(cameraIds: string[], locale: Lang): Promise<string> {
  const url = "https://rt.data.gov.hk/v2/transport/cid/camera";
  try {
    const d = await cached(
      `cid:${cameraIds.join(",")}:${locale}`,
      TTL.traffic,
      () => postJson<Record<string, unknown>>(url, { locale, cameraIdList: cameraIds }),
    );
    const list = (d.results ?? d.cameraIdList ?? d.data) as unknown;
    if (Array.isArray(list) && list.length > 0) {
      const lines = ["Traffic camera snapshots:"];
      for (const c of list as Record<string, unknown>[]) {
        const url2 = (c.url ?? c.snapshotUrl ?? c["image-url"]) as string | undefined;
        lines.push(
          `- camera ${c.cameraId ?? c.camera_id ?? "?"}${c.roadName ?? c.road_name ? ` [${c.roadName ?? c.road_name}]` : ""}: ${url2 ?? JSON.stringify(c)}`,
        );
      }
      return lines.join("\n");
    }
    return `Camera API returned no entries for ids [${cameraIds.join(", ")}]. Raw: ${JSON.stringify(d).slice(0, 500)}`;
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return `Traffic snapshot API unavailable (${msg}). Official endpoint: POST ${url} with body {"locale":"tc","cameraIdList":["A01",...]}. TD camera id prefixes: A (HK Island), KE (Kowloon East), KW, KK, LCK, ST, TW, TY, NT (New Territories). Retry later or check data.gov.hk dataset「交通快拍圖像」.`;
  }
}

interface SpeedRow {
  LINK_ID: string;
  CAPTURE_DATE: string;
  TRAFFIC_SPEED: string;
  ROAD_SATURATION_LEVEL: string;
}

/** Transport Department speed map (XML). The feed has been returning 503
 *  "temporarily unavailable" lately; failures are reported verbatim. */
export async function trafficSpeed(): Promise<string> {
  const url = "https://resource.data.one.gov.hk/td/speedmap.xml";
  let xml: string;
  try {
    xml = await cached("td:speedmap", TTL.traffic, () => fetchText(url));
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return `Traffic speed feed unavailable (${msg}). Official endpoint: ${url} — TD has been returning 503 for this feed; retry later.`;
  }
  const rows: SpeedRow[] = [];
  const re = /<speedmap\s+([^/>]+)\/>/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(xml)) !== null) {
    const attrs = m[1];
    const get = (k: string) => attrs.match(new RegExp(`${k}="([^"]*)"`))?.[1] ?? "";
    rows.push({
      LINK_ID: get("LINK_ID"),
      CAPTURE_DATE: get("CAPTURE_DATE"),
      TRAFFIC_SPEED: get("TRAFFIC_SPEED"),
      ROAD_SATURATION_LEVEL: get("ROAD_SATURATION_LEVEL"),
    });
  }
  if (rows.length === 0) return `Speed map returned no rows. Raw start: ${xml.slice(0, 200)}`;
  const counts = new Map<string, number>();
  for (const r of rows) counts.set(r.ROAD_SATURATION_LEVEL, (counts.get(r.ROAD_SATURATION_LEVEL) ?? 0) + 1);
  const summary = [...counts.entries()].map(([k, v]) => `${k}=${v}`).join(", ");
  const sample = rows
    .slice(0, 30)
    .map((r) => `- link ${r.LINK_ID}: ${r.TRAFFIC_SPEED} km/h [${r.ROAD_SATURATION_LEVEL}] @ ${r.CAPTURE_DATE}`)
    .join("\n");
  return `TD speed map — ${rows.length} road links (${summary}, captured ${rows[0]?.CAPTURE_DATE ?? "?"}).\nFirst 30 links:\n${sample}\n(Full data: ${rows.length} links; LINK_ID geometry is published in the TD speed map dataspec.)`;
}
