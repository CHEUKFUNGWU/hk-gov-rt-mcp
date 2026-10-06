package hkapi

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed data/mtr-stations.json data/lrt-stops.json data/mtr-frequencies.json
var dataFS embed.FS

type stationEntry struct {
	TC string `json:"tc"`
	EN string `json:"en"`
}

type mtrStationsFile struct {
	Lines map[string]map[string]stationEntry `json:"lines"`
}

type lrtEntry struct {
	TC    string   `json:"tc"`
	EN    string   `json:"en"`
	Lines []string `json:"lines"`
}

type lrtStopsFile struct {
	Stops map[string]lrtEntry `json:"stops"`
}

var (
	mtrLines  mtrStationsFile
	lrtStops  lrtStopsFile
	lrtLoaded = sync.OnceFunc(func() {
		raw, err := dataFS.ReadFile("data/mtr-stations.json")
		if err == nil {
			_ = json.Unmarshal(raw, &mtrLines)
		}
		raw, err = dataFS.ReadFile("data/lrt-stops.json")
		if err == nil {
			_ = json.Unmarshal(raw, &lrtStops)
		}
	})
)

func init() { lrtLoaded() }

// LineCodes returns lowercase MTR line codes from the embedded table.
func LineCodes() []string {
	out := make([]string, 0, len(mtrLines.Lines))
	for code := range mtrLines.Lines {
		out = append(out, strings.ToLower(code))
	}
	sort.Strings(out)
	return out
}

func stationName(line, station string, lang Lang) string {
	s, ok := mtrLines.Lines[strings.ToUpper(line)][strings.ToUpper(station)]
	if !ok {
		return strings.ToUpper(station)
	}
	if lang == EN {
		return s.EN
	}
	return s.TC
}

func TrainSchedule(ctx context.Context, line, station string, lang Lang) (string, error) {
	url := fmt.Sprintf("https://rt.data.gov.hk/v1/transport/mtr/getSchedule.php?line=%s&sta=%s", line, station)
	d, err := Cached(ctx, fmt.Sprintf("mtr:%s:%s:%s", line, station, lang), TTLEta, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, url, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return "", err
	}
	if errObj := Obj(d, "error"); errObj != nil {
		return fmt.Sprintf("MTR API error %v — check line/station codes. Valid lines: %s.",
			errObj["errorMsg"], strings.Join(LineCodes(), ", ")), nil
	}
	// status:0 = service notice (special arrangements / suspended station)
	if st, _ := d["status"].(float64); st == 0 {
		if msg := Str(d, "message"); msg != "" {
			if u := Str(d, "url"); u != "" {
				return "MTR service notice: " + msg + "\nInfo: " + u, nil
			}
			return "MTR service notice: " + msg, nil
		}
	}
	lc := strings.ToUpper(line)
	var b strings.Builder
	for key, val := range Obj(d, "data") {
		sta := key
		if i := strings.Index(key, "-"); i >= 0 {
			sta = key[i+1:]
		}
		fmt.Fprintf(&b, "== %s (%s) ==\n", stationName(lc, sta, lang), key)
		stationData, _ := val.(map[string]any)
		if stationData == nil {
			continue
		}
		for _, dir := range []string{"UP", "DOWN"} {
			trains := AsObjList(Arr(stationData, dir))
			if len(trains) == 0 {
				continue
			}
			arrow := "↓"
			if dir == "UP" {
				arrow = "↑"
			}
			parts := []string{}
			for _, t := range trains {
				ttnt := Str(t, "ttnt")
				detail := Str(t, "time")
				if ttnt != "" {
					detail = fmt.Sprintf("%s min", ttnt)
				}
				parts = append(parts, fmt.Sprintf("plat %v → %s @ %s (%s)", t["plat"], stationName(lc, Str(t, "dest"), lang), Str(t, "time"), detail))
			}
			fmt.Fprintf(&b, "%s %s\n", arrow, strings.Join(parts, "; "))
		}
	}
	if d["isdelay"] == true || d["isdelay"] == "Y" {
		b.WriteString("⚠ MTR reports service delay.\n")
	}
	fmt.Fprintf(&b, "(sys_time %s)", Str(d, "sys_time"))
	if b.Len() == 0 {
		return "No schedule data returned.", nil
	}
	return b.String(), nil
}

func LrtSchedule(ctx context.Context, stationID string, lang Lang) (string, error) {
	url := fmt.Sprintf("https://rt.data.gov.hk/v1/transport/mtr/lrt/getSchedule?station_id=%s", stationID)
	d, err := Cached(ctx, fmt.Sprintf("lrt:%s:%s", stationID, lang), TTLEta, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, url, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return "", err
	}
	entry, known := lrtStops.Stops[stationID]
	var title string
	if known {
		name := entry.TC
		if lang == EN {
			name = entry.EN
		}
		title = fmt.Sprintf("LRT %s (%s)", name, stationID)
	} else {
		title = fmt.Sprintf("LRT stop %s", stationID)
	}
	platforms := AsObjList(Arr(d, "platform_list"))
	if len(platforms) == 0 {
		hint := ""
		if !known {
			ids := make([]string, 0, 20)
			i := 0
			for id := range lrtStops.Stops {
				ids = append(ids, id)
				i++
				if i >= 20 {
					break
				}
			}
			sort.Strings(ids)
			hint = fmt.Sprintf(" Unknown station id — well-known ids include %s…", strings.Join(ids, ", "))
		}
		return fmt.Sprintf("%s: no schedule returned (status %s).%s", title, fmt.Sprint(d["status"]), hint), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s — arriving trains:", title)
	for _, p := range platforms {
		for _, r := range AsObjList(Arr(p, "route_list")) {
			t, dest := "time_en", "dest_en"
			if lang != EN {
				t, dest = "time_ch", "dest_ch"
			}
			fmt.Fprintf(&b, "\n- plat %v | route %s → %s: %s", p["platform_id"], Str(r, "route_no"), Str(r, dest), Str(r, t))
		}
	}
	return b.String(), nil
}

func BusSchedule(ctx context.Context, route string) (string, error) {
	url := fmt.Sprintf("https://rt.data.gov.hk/v1/transport/mtr/bus/getSchedule?route=%s", route)
	d, err := Cached(ctx, "mtrbus:"+route, TTLEta, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, url, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return fmt.Sprintf("MTR bus schedule is currently unavailable upstream (%s). The endpoint is officially listed at %s — try again later.", err.Error(), url), nil
	}
	raw, _ := json.MarshalIndent(d, "", " ")
	return fmt.Sprintf("MTR bus route %s:\n%s", route, raw), nil
}

func LookupStationCodes(ctx context.Context, query string, lang Lang) (string, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	var lines []string
	for lc, stations := range mtrLines.Lines {
		for sc, s := range stations {
			name, other := s.TC, s.EN
			if lang == EN {
				name, other = s.EN, s.TC
			}
			if strings.Contains(strings.ToLower(name), q) || strings.Contains(strings.ToLower(other), q) || strings.EqualFold(sc, q) {
				lines = append(lines, fmt.Sprintf("%s line — %s = %s (%s)", lc, sc, name, other))
			}
		}
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No station matching %q. Valid lines: %s", query, strings.Join(LineCodes(), ", ")), nil
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

/* ---------- published average headways (scraped from mtr.com.hk) ---------- */

type freqEntry struct {
	Code      string            `json:"code"`
	LightRail bool              `json:"lightRail"`
	SegmentTc string            `json:"segmentTc"`
	SegmentEn string            `json:"segmentEn"`
	LabelTc   string            `json:"labelTc"`
	LabelEn   string            `json:"labelEn"`
	Bands     map[string]string `json:"bands"`
}

type freqFile struct {
	ScrapedAt string            `json:"scrapedAt"`
	Sources   []string          `json:"sources"`
	BandsTc   map[string]string `json:"bandsTc"`
	BandsEn   map[string]string `json:"bandsEn"`
	NotesTc   string            `json:"notesTc"`
	NotesEn   string            `json:"notesEn"`
	Entries   []freqEntry       `json:"entries"`
}

var freqData freqFile

func init() {
	raw, err := dataFS.ReadFile("data/mtr-frequencies.json")
	if err == nil {
		_ = json.Unmarshal(raw, &freqData)
	}
}

var bandOrder = []string{"amPeak", "pmPeak", "offPeak", "saturday", "sundayHoliday"}

var bandShort = map[Lang]map[string]string{
	TC: {"amPeak": "平日朝繁", "pmPeak": "平日晚繁", "offPeak": "非繁忙", "saturday": "週六", "sundayHoliday": "假日"},
	SC: {"amPeak": "平日朝高峰", "pmPeak": "平日晚高峰", "offPeak": "非高峰", "saturday": "週六", "sundayHoliday": "假日"},
	EN: {"amPeak": "AM peak", "pmPeak": "PM peak", "offPeak": "Off-peak", "saturday": "Sat", "sundayHoliday": "Sun&PH"},
}

// MtrFrequency returns the published average headways table, optionally
// filtered by line code / LRT route / name fragment. Static snapshot.
func MtrFrequency(line string, lang Lang) string {
	t := lang == EN
	all := freqData.Entries
	list := all
	if q := strings.ToUpper(strings.TrimSpace(line)); q != "" {
		list = nil
		for _, e := range all {
			code := strings.ToUpper(e.Code)
			match := code == q || code == "LRT-"+strings.TrimPrefix(q, "LRT-")
			if !match {
				name := strings.ToLower(e.LabelTc + e.LabelEn + e.SegmentTc + e.SegmentEn)
				match = strings.Contains(name, strings.ToLower(strings.TrimSpace(line)))
			}
			if match {
				list = append(list, e)
			}
		}
		if len(list) == 0 {
			codes := map[string]bool{}
			for _, e := range all {
				if e.Code != "" {
					codes[e.Code] = true
				}
			}
			keys := make([]string, 0, len(codes))
			for k := range codes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Sprintf("No frequency entry matching %q. Valid codes: %s.", line, strings.Join(keys, ", "))
		}
	}
	bands := freqData.BandsTc
	if t {
		bands = freqData.BandsEn
	}
	short := bandShort[lang]
	header := fmt.Sprintf("MTR published average headways (minutes) — static snapshot scraped %s from mtr.com.hk (not real-time; use get_mtr_schedule / get_lrt_schedule for live arrivals)", freqData.ScrapedAt)
	if !t {
		header = fmt.Sprintf("港鐵公佈嘅平均班次（分鐘）— %s 從 mtr.com.hk 擷取嘅靜態快照（並非實時；實時到站請用 get_mtr_schedule / get_lrt_schedule）", freqData.ScrapedAt)
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(bands["amPeak"] + " | " + bands["pmPeak"] + " | " + bands["offPeak"] + " | " + bands["saturday"] + " | " + bands["sundayHoliday"])
	for _, e := range list {
		nameEn := e.SegmentEn
		if nameEn == "" {
			nameEn = e.LabelEn
		}
		nameTc := e.SegmentTc
		if nameTc == "" {
			nameTc = e.LabelTc
		}
		label := e.Code + " "
		if t {
			label += nameEn
		} else {
			label += nameTc
			if nameEn != "" && nameEn != nameTc {
				label += " / " + nameEn
			}
		}
		label += ": "
		cells := make([]string, 0, len(bandOrder))
		for _, k := range bandOrder {
			v := e.Bands[k]
			if v == "" {
				v = "-"
			}
			cells = append(cells, short[k]+" "+v)
		}
		b.WriteString("\n" + label + strings.Join(cells, " | "))
	}
	if t {
		b.WriteString("\n\n" + freqData.NotesEn)
		b.WriteString("\nSource: " + freqData.Sources[0])
	} else {
		b.WriteString("\n\n" + freqData.NotesTc)
		b.WriteString("\n來源: " + freqData.Sources[0])
	}
	return b.String()
}
