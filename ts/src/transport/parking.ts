import { fetchJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { type Lang } from "../lang.js";

type Row = Record<string, unknown>;

const BASIC_URL = "https://resource.data.one.gov.hk/td/carpark/basic_info_all.json";
const VACANCY_URL = "https://resource.data.one.gov.hk/td/carpark/vacancy_all.json";

const VEHICLE_LABEL: Record<Lang, Record<string, string>> = {
  tc: { P: "私家車", P_D: "私家車(殘疾)", M: "電單車", LGV: "輕型貨車", HGV: "重型貨車", COACH: "旅遊巴士", N: "貨車" },
  sc: { P: "私家车", P_D: "私家车(残疾)", M: "摩托车", LGV: "轻型货车", HGV: "重型货车", COACH: "旅游巴士", N: "货车" },
  en: { P: "private car", P_D: "private car (disabled)", M: "motorcycle", LGV: "LGV", HGV: "HGV", COACH: "coach", N: "goods vehicle" },
};

function vacancyLabel(v: number, lang: Lang): string {
  if (v === 1000) return lang === "en" ? "FULL" : "已滿";
  if (v === 1001 || v < 0) return lang === "en" ? "N/A" : "不適用";
  return String(v);
}

interface Carpark {
  park_id: string;
  name: string;
  displayAddress: string;
  district: string;
  lat?: number;
  lon?: number;
  vacancies: string[];
}

export async function parkingVacancy(keyword: string | undefined, lang: Lang): Promise<string> {
  const [basic, vac] = await Promise.all([
    cached("td:carpark:basic", TTL.parking, () => fetchJson<{ car_park: Row[] }>(BASIC_URL)),
    cached("td:carpark:vacancy", TTL.parking, () => fetchJson<{ car_park: Row[] }>(VACANCY_URL)),
  ]);
  const info = new Map<string, Row>();
  for (const c of basic.car_park ?? []) info.set(String(c.park_id), c);
  const vmap = new Map<string, Row[]>();
  for (const c of vac.car_park ?? []) vmap.set(String(c.park_id), (c.vehicle_type as Row[]) ?? []);

  const q = keyword?.trim().toLowerCase();
  const results: Carpark[] = [];
  for (const [parkId, vts] of vmap) {
    const b = info.get(parkId);
    const name = b ? String(b[`name_${lang}`] ?? b.name_en ?? parkId) : parkId;
    const district = b ? String(b[`district_${lang}`] ?? b.district_en ?? "") : "";
    const displayAddress = b ? String(b[`displayAddress_${lang}`] ?? b.displayAddress_en ?? "") : "";
    if (q && !name.toLowerCase().includes(q) && !district.toLowerCase().includes(q) && !displayAddress.toLowerCase().includes(q)) {
      continue;
    }
    const vacancies: string[] = [];
    for (const vt of vts) {
      const label = VEHICLE_LABEL[lang][String(vt.type)] ?? String(vt.type);
      for (const sc of (vt.service_category as Row[]) ?? []) {
        vacancies.push(
          `${label} [${String(sc.category)}]: ${vacancyLabel(Number(sc.vacancy), lang)} (${sc.lastupdate ?? ""})`,
        );
      }
    }
    if (vacancies.length === 0) continue;
    results.push({ park_id: parkId, name, displayAddress, district, lat: b?.latitude as number | undefined, lon: b?.longitude as number | undefined, vacancies });
  }
  if (results.length === 0) {
    return `No participating carparks with live vacancy${q ? ` matching "${keyword}"` : ""}. The feed only covers TD participating carparks.`;
  }
  const shown = results.slice(0, 30);
  const lines = shown.map((c) => {
    const geo = c.lat !== undefined ? ` @ ${c.lat.toFixed(4)},${c.lon?.toFixed(4)}` : "";
    return `## ${c.name} (${c.park_id})${geo}\n   ${c.displayAddress}\n   ` + c.vacancies.join("\n   ");
  });
  return `${results.length} participating carpark(s) with live vacancy${q ? ` matching "${keyword}"` : ""}${results.length > 30 ? ` (showing first 30)` : ""}:\n\n${lines.join("\n\n")}`;
}
