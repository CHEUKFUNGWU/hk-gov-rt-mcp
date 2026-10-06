import { fetchJson } from "../http.js";
import { cached, TTL } from "../cache.js";
import { pick, type Lang } from "../lang.js";

const BASE = "https://data.weather.gov.hk/weatherAPI/opendata/weather.php";

async function hko(dataType: string, lang: Lang, ttlMs: number): Promise<Record<string, unknown>> {
  return cached(`hko:${dataType}:${lang}`, ttlMs, () =>
    fetchJson<Record<string, unknown>>(`${BASE}?dataType=${dataType}&lang=${lang}`),
  );
}

export async function currentWeather(lang: Lang): Promise<Record<string, unknown>> {
  return hko("rhrread", lang, TTL.weatherNow);
}

export async function localForecast(lang: Lang): Promise<Record<string, unknown>> {
  return hko("flw", lang, TTL.forecast);
}

export async function nineDayForecast(lang: Lang): Promise<Record<string, unknown>> {
  return hko("fnd", lang, TTL.forecast);
}

/** Merge warning summary + warning details + special weather tips. The three
 *  upstream payloads are empty objects/arrays when nothing is in force, and
 *  their active shapes vary; normalize defensively. */
export async function weatherWarnings(lang: Lang): Promise<Record<string, unknown>> {
  const [sum, info, tips] = await Promise.all([
    hko("warnsum", lang, TTL.warnings),
    hko("warningInfo", lang, TTL.warnings),
    hko("swt", lang, TTL.warnings),
  ]);
  const arrays: unknown[] = [];
  const extra: Record<string, unknown> = {};
  for (const [name, payload] of [
    ["warningSummary", sum],
    ["warningInfo", info],
    ["specialTips", tips],
  ] as const) {
    if (Array.isArray(payload)) {
      arrays.push(...payload);
    } else if (payload && typeof payload === "object") {
      let nested = false;
      for (const [k, v] of Object.entries(payload)) {
        if (Array.isArray(v) && v.length > 0) {
          arrays.push(...v);
          nested = true;
        }
      }
      if (!nested && Object.keys(payload).length > 0) extra[name] = payload;
    }
  }
  return { warnings: arrays, ...extra };
}

/* ---------- formatters ---------- */

export async function formatCurrent(lang: Lang): Promise<string> {
  const d = await currentWeather(lang);
  const temps = Array.isArray(d.temperature) ? (d.temperature as Record<string, unknown>[]) : [];
  const hums = Array.isArray(d.humidity) ? (d.humidity as Record<string, unknown>[]) : [];
  const rains = Array.isArray(d.rainfall) ? (d.rainfall as Record<string, unknown>[]) : [];
  const uvs = Array.isArray(d.uvindex) ? (d.uvindex as Record<string, unknown>[]) : [];
  const lines: string[] = [];
  lines.push(`HKO current weather (updateTime: ${String(d.updateTime ?? "-")})`);
  const icons = (d.icon as number[] | undefined) ?? [];
  if (icons.length > 0) lines.push(`Weather icon(s): ${icons.join(", ")}`);
  if (temps.length > 0) {
    lines.push(
      "Temperature: " +
        temps.map((t) => `${t.place} ${t.value}${t.unit ?? "°C"}`).join(", "),
    );
  }
  if (hums.length > 0) {
    lines.push("Humidity: " + hums.map((h) => `${h.place} ${h.value}${h.unit ?? "%"}`).join(", "));
  }
  const wet = rains.filter((r) => Number(r.max ?? r.value ?? 0) > 0);
  if (rains.length > 0) {
    lines.push(
      "Rainfall (past hour): " +
        (wet.length > 0
          ? wet.map((r) => `${r.place} ${r.max ?? r.value}${r.unit ?? "mm"}`).join(", ")
          : `${lang === "en" ? "none recorded" : lang === "tc" ? "各區無雨" : "各区无雨"}`),
    );
  }
  if (uvs.length > 0) {
    lines.push(
      "UV index: " + uvs.map((u) => `${u.place} ${u.value}${u.desc ? ` (${u.desc})` : ""}`).join(", "),
    );
  }
  const warn = d.warningMessage;
  if (Array.isArray(warn) && warn.length > 0) lines.push(`Warnings: ${warn.join("; ")}`);
  if (typeof d.tcmessage === "string" && d.tcmessage.length > 0) lines.push(`TC message: ${d.tcmessage}`);
  for (const k of ["mintempFrom00To09", "maxtempFrom00To09", "rainfallFrom00To12", "rainfallThisMonth"] as const) {
    if (d[k] !== null && d[k] !== undefined && d[k] !== "") lines.push(`${k}: ${String(d[k])}`);
  }
  return lines.join("\n");
}

export async function formatLocal(lang: Lang): Promise<string> {
  const d = await localForecast(lang);
  return [
    `HKO local forecast (updateTime: ${String(d.updateTime ?? "-")})`,
    `General situation: ${String(d.generalSituation ?? "-")}`,
    `Tropical cyclone info: ${String(d.tcInfo ?? "-")}`,
    `Fire danger warning: ${String(d.fireDangerWarning ?? "-")}`,
    `Forecast (${String(d.forecastPeriod ?? "-")}): ${String(d.forecastDesc ?? "-")}`,
    `Outlook: ${String(d.outlook ?? "-")}`,
  ].join("\n");
}

export async function formatNineDay(lang: Lang): Promise<string> {
  const d = await nineDayForecast(lang);
  const days = (d.weatherForecast as Record<string, unknown>[] | undefined) ?? [];
  const rows = days.map((f) => {
    const max = (f.forecastMaxtemp as Record<string, unknown> | undefined)?.value;
    const min = (f.forecastMintemp as Record<string, unknown> | undefined)?.value;
    const maxrh = (f.forecastMaxrh as Record<string, unknown> | undefined)?.value;
    const minrh = (f.forecastMinrh as Record<string, unknown> | undefined)?.value;
    return `${f.forecastDate} (${f.week}): ${f.weather}, ${min}–${max}°C, RH ${minrh}–${maxrh}%, wind ${f.wind}, PSR ${f.PSR ?? "-"}`;
  });
  const sea = d.seaTemp as Record<string, unknown> | undefined;
  const lines = [
    `HKO 9-day forecast (updateTime: ${String(d.updateTime ?? "-")})`,
    `General situation: ${String(d.generalSituation ?? "-")}`,
    ...rows,
  ];
  if (sea) lines.push(`Sea temp: ${sea.place} ${sea.value}${sea.unit ?? "°C"} (${sea.recordTime ?? ""})`);
  return lines.join("\n");
}

export async function formatWarnings(lang: Lang): Promise<string> {
  const d = await weatherWarnings(lang);
  const warnings = (d.warnings as unknown[]) ?? [];
  const tips = d.specialTips;
  const extraKeys = Object.keys(d).filter((k) => k !== "warnings" && k !== "specialTips");
  if (warnings.length === 0 && extraKeys.length === 0 && (tips === undefined || tips === "")) {
    return lang === "en"
      ? "No weather warnings in force; no special tips."
      : lang === "tc"
        ? "現時沒有生效的天气警告，亦無特別天氣提示。"
        : "现时没有生效的天气警告，亦无特别天气提示。";
  }
  const lines: string[] = [];
  if (warnings.length > 0) {
    lines.push("Warnings in force:");
    for (const w of warnings) lines.push(`- ${JSON.stringify(w)}`);
  }
  for (const k of extraKeys) lines.push(`${k}: ${JSON.stringify(d[k])}`);
  if (Array.isArray(tips)) {
    for (const t of tips as Record<string, unknown>[]) {
      if (typeof t?.Desc === "string" && t.Desc) lines.push(`Special tips: ${t.Desc} (${t.UpdateTime ?? ""})`);
    }
  }
  return lines.join("\n");
}

export { pick };
