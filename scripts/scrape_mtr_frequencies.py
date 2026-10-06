#!/usr/bin/env python3
"""Scrape MTR average train frequencies from the official website.

Produces mtr-frequencies.json (bilingual) into both implementations'
embedded-data folders. Re-run whenever MTR updates its published table:
    python3 scripts/scrape_mtr_frequencies.py

Data source: https://www.mtr.com.hk/ch/customer/services/train_service_index.html
(and the /en mirror). Scraped snapshot — check the "scrapedAt" field.
"""
import datetime
import html
import json
import re
import urllib.request

UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36"
URLS = {
    "tc": "https://www.mtr.com.hk/ch/customer/services/train_service_index.html",
    "en": "https://www.mtr.com.hk/en/customer/services/train_service_index.html",
}
BAND_KEYS = ["amPeak", "pmPeak", "offPeak", "saturday", "sundayHoliday"]

LINE_CODES = {
    "港島綫": "ISL", "荃灣綫": "TWL", "觀塘綫": "KTL", "將軍澳綫": "TKL",
    "南港島綫": "SIL", "東涌綫": "TCL", "迪士尼綫": "DRL", "屯馬綫": "TML",
    "東鐵綫": "EAL", "機場快綫": "AEL",
    "Island Line": "ISL", "Tsuen Wan Line": "TWL", "Kwun Tong Line": "KTL",
    "Tseung Kwan O Line": "TKL", "South Island Line": "SIL",
    "Tung Chung Line": "TCL", "Disneyland Resort Line": "DRL",
    "Tuen Ma Line": "TML", "East Rail Line": "EAL", "Airport Express": "AEL",
}
LIGHT_RAIL_HEADERS = {"輕鐵", "Light Rail"}


def fetch(url: str) -> str:
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=30) as resp:
        return resp.read().decode("utf-8", "replace")


def parse_rows(page: str):
    tables = re.findall(r"<table.*?</table>", page, flags=re.S)
    assert tables, "no frequency table found"
    rows = []
    for tr in re.findall(r"<tr.*?</tr>", tables[0], flags=re.S):
        cells = []
        for c in re.findall(r"<t[dh][^>]*>(.*?)</t[dh]>", tr, flags=re.S):
            c = html.unescape(re.sub(r"<[^>]+>", "", c)).replace("\u00a0", " ")
            cells.append(re.sub(r"\s+", " ", c).strip())
        if any(cells):
            rows.append(cells)
    return rows[1:]  # drop the band-header row


def scrape(lang: str):
    """Return ordered entries: {label, code, lightRail, bands} — bands may be
    None for pure line-header rows (segments follow)."""
    entries = []
    current = None
    light_rail = False
    for cells in parse_rows(fetch(URLS[lang])):
        label = cells[0].strip()
        values = cells[1:6] + [""] * (5 - len(cells[1:6]))
        has_values = any(v not in ("", "-", "–") for v in values)
        if label in LIGHT_RAIL_HEADERS:
            light_rail = True
            current = None
            continue
        key = re.sub(r"[*~#\s]+$", "", label)
        if not light_rail and key in LINE_CODES:
            current = LINE_CODES[key]
            if has_values:
                entries.append({"label": label, "code": current, "lightRail": False,
                                "segment": None, "bands": dict(zip(BAND_KEYS, values))})
                current = None
            continue
        if light_rail:
            m = re.search(r"(\d{3}[A-Za-z]{0,2}P?)", label)
            code = ("LRT-" + m.group(1).upper()) if m else "LRT"
            entries.append({"label": label, "code": code, "lightRail": True,
                            "segment": None, "bands": dict(zip(BAND_KEYS, values)) if has_values else None})
            continue
        if current and has_values:
            entries.append({"label": label, "code": current, "lightRail": False,
                            "segment": label, "bands": dict(zip(BAND_KEYS, values))})
            continue
        # unplaced row (rare) — keep as unlabeled fallback so index pairing survives
        entries.append({"label": label, "code": None, "lightRail": False,
                        "segment": label if has_values else None,
                        "bands": dict(zip(BAND_KEYS, values)) if has_values else None})
    return entries


def clean(name: str):
    if name is None:
        return None
    return re.sub(r"\s*[*~#]\s*$", "", name).strip()


def main():
    tc = scrape("tc")
    en = scrape("en")
    print("rows tc/en:", len(tc), len(en))
    for a, b in zip(tc, en):
        if a["label"] != b["label"] and not a["label"].startswith("路綫") and not b["label"].startswith("Route"):
            pass  # labels differ by language by design; index pairing only
    if len(tc) != len(en):
        print("WARN: row count mismatch — en labels will be omitted")

    entries = []
    for i, e in enumerate(tc):
        peer = en[i] if i < len(en) else None
        entries.append({
            "code": e["code"],
            "lightRail": e["lightRail"],
            "segmentTc": clean(e["segment"]),
            "segmentEn": clean(peer["segment"]) if peer else None,
            "labelTc": clean(e["label"]),
            "labelEn": clean(peer["label"]) if peer else None,
            "bands": e["bands"],
        })

    result = {
        "scrapedAt": datetime.date.today().isoformat(),
        "sources": list(URLS.values()),
        "bandsTc": {"amPeak": "平日早上繁忙時段", "pmPeak": "平日晚上繁忙時段",
                    "offPeak": "平日非繁忙時段", "saturday": "星期六",
                    "sundayHoliday": "星期日及公眾假期"},
        "bandsEn": {"amPeak": "Weekday AM peak", "pmPeak": "Weekday PM peak",
                    "offPeak": "Weekday off-peak", "saturday": "Saturday",
                    "sundayHoliday": "Sunday & public holidays"},
        "notesTc": "單位為分鐘（平均班次）。於清晨及深夜時段，港島綫、荃灣綫、觀塘綫、將軍澳綫、南港島綫、東涌綫、屯馬綫、東鐵綫(金鐘–上水)及機場快綫列車最多12分鐘一班；東鐵綫(上水–羅湖/落馬洲)最多21分鐘一班；部分輕鐵路綫最多25分鐘一班。迪士尼綫於樂園關閉期間維持每10–20分鐘一班。",
        "notesEn": "Values in minutes (average headway). In early-morning and late-night periods, trains on Island, Tsuen Wan, Kwun Tong, Tseung Kwan O, South Island, Tung Chung and Tuen Ma lines, East Rail Line (Admiralty–Sheung Shui) and Airport Express run at most every 12 minutes; East Rail (Sheung Shui–Lo Wu/Lok Ma Chau) at most every 21 minutes; some Light Rail routes at most every 25 minutes. Disneyland Resort Line runs every 10–20 minutes while the park is closed.",
        "entries": entries,
    }
    for dest in ("ts/src/transport/data/mtr-frequencies.json", "go/internal/hkapi/data/mtr-frequencies.json"):
        with open(dest, "w", encoding="utf-8") as f:
            json.dump(result, f, ensure_ascii=False, separators=(",", ":"))
        print("wrote", dest)
    print(json.dumps(entries, ensure_ascii=False, indent=1)[:3000])


if __name__ == "__main__":
    main()
