import { z } from "zod";

export const LANGS = ["tc", "sc", "en"] as const;
export type Lang = (typeof LANGS)[number];

export const langParam = z
  .enum(LANGS)
  .default("tc")
  .describe("Output language: tc=繁體中文, sc=简体中文, en=English");

/** HKO APIs use lang=tc|sc|en directly. */
export function hkoLang(l: Lang): Lang {
  return l;
}

/** Pick a localized field following HK open data naming: name_tc / name_sc / name_en. */
export function pick<T extends Record<string, unknown>>(obj: T, base: string, lang: Lang): string {
  const v = obj[`${base}_${lang}`];
  if (typeof v === "string" && v.length > 0) return v;
  const en = obj[`${base}_en`];
  return typeof en === "string" ? en : "";
}
