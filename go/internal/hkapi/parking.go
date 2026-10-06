package hkapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	carparkBasicURL   = "https://resource.data.one.gov.hk/td/carpark/basic_info_all.json"
	carparkVacancyURL = "https://resource.data.one.gov.hk/td/carpark/vacancy_all.json"
)

var vehicleLabel = map[Lang]map[string]string{
	TC: {"P": "私家車", "P_D": "私家車(殘疾)", "M": "電單車", "LGV": "輕型貨車", "HGV": "重型貨車", "COACH": "旅遊巴士", "N": "貨車"},
	SC: {"P": "私家车", "P_D": "私家车(残疾)", "M": "摩托车", "LGV": "轻型货车", "HGV": "重型货车", "COACH": "旅游巴士", "N": "货车"},
	EN: {"P": "private car", "P_D": "private car (disabled)", "M": "motorcycle", "LGV": "LGV", "HGV": "HGV", "COACH": "coach", "N": "goods vehicle"},
}

func vacancyLabel(v int64, lang Lang) string {
	switch {
	case v == 1000:
		if lang == EN {
			return "FULL"
		}
		return "已滿"
	case v == 1001 || v < 0:
		if lang == EN {
			return "N/A"
		}
		return "不適用"
	}
	return fmt.Sprint(v)
}

type carparkHit struct {
	parkID     string
	name       string
	address    string
	district   string
	geo        string
	vacancies  []string
}

// ParkingVacancy joins the TD basic-info and vacancy feeds by park_id.
func ParkingVacancy(ctx context.Context, keyword string, lang Lang) (string, error) {
	type basicFile struct {
		CarPark []map[string]any `json:"car_park"`
	}
	type vacancyFile struct {
		CarPark []map[string]any `json:"car_park"`
	}
	basic, err := Cached(ctx, "td:carpark:basic", TTLParking, func() (*basicFile, error) {
		var out basicFile
		if err := GetJSON(ctx, carparkBasicURL, 15*time.Second, &out); err != nil {
			return nil, err
		}
		return &out, nil
	})
	if err != nil {
		return "", err
	}
	vac, err := Cached(ctx, "td:carpark:vacancy", TTLParking, func() (*vacancyFile, error) {
		var out vacancyFile
		if err := GetJSON(ctx, carparkVacancyURL, 15*time.Second, &out); err != nil {
			return nil, err
		}
		return &out, nil
	})
	if err != nil {
		return "", err
	}

	info := map[string]map[string]any{}
	for _, c := range basic.CarPark {
		info[fmt.Sprint(c["park_id"])] = c
	}
	q := strings.ToLower(strings.TrimSpace(keyword))
	var results []carparkHit
	for _, c := range vac.CarPark {
		parkID := fmt.Sprint(c["park_id"])
		b := info[parkID]
		name := parkID
		district, display := "", ""
		var lat, lon float64
		if b != nil {
			name = Str(b, "name_"+string(lang))
			if name == "" {
				name = Str(b, "name_en")
			}
			district = Str(b, "district_"+string(lang))
			if district == "" {
				district = Str(b, "district_en")
			}
			display = Str(b, "displayAddress_"+string(lang))
			if display == "" {
				display = Str(b, "displayAddress_en")
			}
			lat, _ = b["latitude"].(float64)
			lon, _ = b["longitude"].(float64)
		}
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(strings.ToLower(district), q) && !strings.Contains(strings.ToLower(display), q) {
			continue
		}
		hit := carparkHit{parkID: parkID, name: name, address: display, district: district}
		if b != nil {
			hit.geo = fmt.Sprintf(" @ %.4f,%.4f", lat, lon)
		}
		for _, vt := range AsObjList(Arr(c, "vehicle_type")) {
			label := vehicleLabel[lang][Str(vt, "type")]
			if label == "" {
				label = Str(vt, "type")
			}
			for _, sc := range AsObjList(Arr(vt, "service_category")) {
				v, _ := sc["vacancy"].(float64)
				hit.vacancies = append(hit.vacancies, fmt.Sprintf("%s [%s]: %s (%s)", label, Str(sc, "category"), vacancyLabel(int64(v), lang), Str(sc, "lastupdate")))
			}
		}
		if len(hit.vacancies) > 0 {
			results = append(results, hit)
		}
	}
	if len(results) == 0 {
		return fmt.Sprintf("No participating carparks with live vacancy%s. The feed only covers TD participating carparks.", routeSuffix(keyword)), nil
	}
	shown := results
	truncated := false
	if len(shown) > 30 {
		shown = shown[:30]
		truncated = true
	}
	var b strings.Builder
	suffix := routeSuffix(keyword)
	if truncated {
		fmt.Fprintf(&b, "%d participating carpark(s) with live vacancy%s (showing first 30):\n\n", len(results), suffix)
	} else {
		fmt.Fprintf(&b, "%d participating carpark(s) with live vacancy%s:\n\n", len(results), suffix)
	}
	for i, c := range shown {
		fmt.Fprintf(&b, "## %s (%s)%s\n   %s\n   %s", c.name, c.parkID, c.geo, c.address, strings.Join(c.vacancies, "\n   "))
		if i != len(shown)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String(), nil
}

// jsonMarshal is a tiny helper for defensive raw dumps.
func jsonMarshal(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
