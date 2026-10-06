package hkapi

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const hkoBase = "https://data.weather.gov.hk/weatherAPI/opendata/weather.php"

func hko(ctx context.Context, dataType string, lang Lang, ttl time.Duration) (map[string]any, error) {
	return Cached(ctx, "hko:"+dataType+":"+string(lang), ttl, func() (map[string]any, error) {
		var out map[string]any
		url := fmt.Sprintf("%s?dataType=%s&lang=%s", hkoBase, dataType, lang)
		if err := GetJSON(ctx, url, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
}

func CurrentWeather(ctx context.Context, lang Lang) (map[string]any, error) {
	return hko(ctx, "rhrread", lang, TTLWeatherNow)
}

func LocalForecast(ctx context.Context, lang Lang) (map[string]any, error) {
	return hko(ctx, "flw", lang, TTLForecast)
}

func NineDayForecast(ctx context.Context, lang Lang) (map[string]any, error) {
	return hko(ctx, "fnd", lang, TTLForecast)
}

// WeatherWarnings merges warnsum + warningInfo + swt; empty payloads are
// normal when no warning is in force.
func WeatherWarnings(ctx context.Context, lang Lang) (map[string]any, error) {
	sum, err := hko(ctx, "warnsum", lang, TTLWarnings)
	if err != nil {
		return nil, err
	}
	info, err := hko(ctx, "warningInfo", lang, TTLWarnings)
	if err != nil {
		return nil, err
	}
	tips, err := hko(ctx, "swt", lang, TTLWarnings)
	if err != nil {
		return nil, err
	}
	warnings := []any{}
	extra := map[string]any{}
	for _, kv := range []struct {
		name    string
		payload map[string]any
	}{
		{"warningSummary", sum}, {"warningInfo", info}, {"specialTips", tips},
	} {
		nested := false
		for _, v := range kv.payload {
			if arr, ok := v.([]any); ok && len(arr) > 0 {
				warnings = append(warnings, arr...)
				nested = true
			}
		}
		if !nested && len(kv.payload) > 0 {
			extra[kv.name] = kv.payload
		}
	}
	out := map[string]any{"warnings": warnings}
	for k, v := range extra {
		out[k] = v
	}
	return out, nil
}

/* ---------- formatters (mirror the TypeScript outputs) ---------- */

func FormatCurrent(ctx context.Context, lang Lang) (string, error) {
	d, err := CurrentWeather(ctx, lang)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "HKO current weather (updateTime: %s)", Str(d, "updateTime"))
	if icons, ok := d["icon"].([]any); ok && len(icons) > 0 {
		fmt.Fprintf(&b, "\nWeather icon(s): %v", icons)
	}
	if temps := AsObjList(Arr(d, "temperature")); len(temps) > 0 {
		parts := []string{}
		for _, t := range temps {
			unit := Str(t, "unit")
			if unit == "" {
				unit = "°C"
			}
			parts = append(parts, fmt.Sprintf("%s %v%s", Str(t, "place"), t["value"], unit))
		}
		fmt.Fprintf(&b, "\nTemperature: %s", strings.Join(parts, ", "))
	}
	if hums := AsObjList(Arr(d, "humidity")); len(hums) > 0 {
		parts := []string{}
		for _, h := range hums {
			unit := Str(h, "unit")
			if unit == "" {
				unit = "%"
			}
			parts = append(parts, fmt.Sprintf("%s %v%s", Str(h, "place"), h["value"], unit))
		}
		fmt.Fprintf(&b, "\nHumidity: %s", strings.Join(parts, ", "))
	}
	if rains := AsObjList(Arr(d, "rainfall")); len(rains) > 0 {
		wet := []string{}
		anyRain := false
		for _, r := range rains {
			max := r["max"]
			if f, ok := max.(float64); ok && f > 0 {
				anyRain = true
				unit := Str(r, "unit")
				if unit == "" {
					unit = "mm"
				}
				wet = append(wet, fmt.Sprintf("%s %v%s", Str(r, "place"), max, unit))
			}
		}
		if anyRain {
			fmt.Fprintf(&b, "\nRainfall (past hour): %s", strings.Join(wet, ", "))
		} else {
			switch lang {
			case EN:
				fmt.Fprint(&b, "\nRainfall (past hour): none recorded")
			case SC:
				fmt.Fprint(&b, "\nRainfall (past hour): 各区无雨")
			default:
				fmt.Fprint(&b, "\nRainfall (past hour): 各區無雨")
			}
		}
	}
	if uvs := AsObjList(Arr(d, "uvindex")); len(uvs) > 0 {
		parts := []string{}
		for _, u := range uvs {
			desc := Str(u, "desc")
			if desc != "" {
				desc = " (" + desc + ")"
			}
			parts = append(parts, fmt.Sprintf("%s %v%s", Str(u, "place"), u["value"], desc))
		}
		fmt.Fprintf(&b, "\nUV index: %s", strings.Join(parts, ", "))
	}
	if warn, ok := d["warningMessage"].([]any); ok && len(warn) > 0 {
		fmt.Fprintf(&b, "\nWarnings: %v", warn)
	}
	if tc := Str(d, "tcmessage"); tc != "" {
		fmt.Fprintf(&b, "\nTC message: %s", tc)
	}
	for _, k := range []string{"mintempFrom00To09", "maxtempFrom00To09", "rainfallFrom00To12", "rainfallThisMonth"} {
		if v, ok := d[k]; ok && v != nil && v != "" {
			fmt.Fprintf(&b, "\n%s: %v", k, v)
		}
	}
	return b.String(), nil
}

func FormatLocal(ctx context.Context, lang Lang) (string, error) {
	d, err := LocalForecast(ctx, lang)
	if err != nil {
		return "", err
	}
	fallback := func(m map[string]any, key string) string {
		s := Str(m, key)
		if s == "" {
			return "-"
		}
		return s
	}
	return strings.Join([]string{
		fmt.Sprintf("HKO local forecast (updateTime: %s)", fallback(d, "updateTime")),
		"General situation: " + fallback(d, "generalSituation"),
		"Tropical cyclone info: " + fallback(d, "tcInfo"),
		"Fire danger warning: " + fallback(d, "fireDangerWarning"),
		fmt.Sprintf("Forecast (%s): %s", fallback(d, "forecastPeriod"), fallback(d, "forecastDesc")),
		"Outlook: " + fallback(d, "outlook"),
	}, "\n"), nil
}

func FormatNineDay(ctx context.Context, lang Lang) (string, error) {
	d, err := NineDayForecast(ctx, lang)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "HKO 9-day forecast (updateTime: %s)\nGeneral situation: %s",
		Str(d, "updateTime"), Str(d, "generalSituation"))
	for _, f := range AsObjList(Arr(d, "weatherForecast")) {
		maxT := Obj(f, "forecastMaxtemp")["value"]
		minT := Obj(f, "forecastMintemp")["value"]
		maxRH := Obj(f, "forecastMaxrh")["value"]
		minRH := Obj(f, "forecastMinrh")["value"]
		psr := Str(f, "PSR")
		if psr == "" {
			psr = "-"
		}
		fmt.Fprintf(&b, "\n%s (%s): %s, %v–%v°C, RH %v–%v%%, wind %s, PSR %s",
			Str(f, "forecastDate"), Str(f, "week"), Str(f, "weather"), minT, maxT, minRH, maxRH, Str(f, "wind"), psr)
	}
	if sea := Obj(d, "seaTemp"); sea != nil {
		fmt.Fprintf(&b, "\nSea temp: %s %v%s (%s)", Str(sea, "place"), sea["value"], Str(sea, "unit"), Str(sea, "recordTime"))
	}
	return b.String(), nil
}

func FormatWarnings(ctx context.Context, lang Lang) (string, error) {
	d, err := WeatherWarnings(ctx, lang)
	if err != nil {
		return "", err
	}
	warnings := Arr(d, "warnings")
	tips := d["specialTips"]
	hasTips := false
	if arr, ok := tips.([]any); ok {
		for _, t := range AsObjList(arr) {
			if Str(t, "Desc") != "" {
				hasTips = true
			}
		}
	}
	if len(warnings) == 0 && len(d) <= 1 && !hasTips {
		switch lang {
		case EN:
			return "No weather warnings in force; no special tips.", nil
		case SC:
			return "现时没有生效的天气警告，亦无特别天气提示。", nil
		default:
			return "現時沒有生效的天气警告，亦無特別天氣提示。", nil
		}
	}
	var b strings.Builder
	if len(warnings) > 0 {
		b.WriteString("Warnings in force:")
		for _, w := range warnings {
			fmt.Fprintf(&b, "\n- %v", w)
		}
	}
	for k, v := range d {
		if k == "warnings" || k == "specialTips" {
			continue
		}
		fmt.Fprintf(&b, "\n%s: %v", k, v)
	}
	for _, t := range AsObjList(Arr(d, "specialTips")) {
		if desc := Str(t, "Desc"); desc != "" {
			fmt.Fprintf(&b, "\nSpecial tips: %s (%s)", desc, Str(t, "UpdateTime"))
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
