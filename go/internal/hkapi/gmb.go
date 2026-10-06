package hkapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const gmbBase = "https://data.etagmb.gov.hk"

func gmbGet(ctx context.Context, path string, ttl time.Duration) (map[string]any, error) {
	return Cached(ctx, "gmb:"+path, ttl, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, gmbBase+path, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
}

// numStr renders JSON numbers (decoded as float64) without scientific notation.
func numStr(v any) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func RouteList(ctx context.Context, region, route string) (string, error) {
	all, err := gmbGet(ctx, "/route", TTLStaticRoute)
	if err != nil {
		return "", err
	}
	routes, _ := Obj(all, "data")["routes"].(map[string]any)
	q := strings.ToLower(strings.TrimSpace(route))
	var lines []string
	for reg, v := range routes {
		if region != "" && reg != strings.ToUpper(region) {
			continue
		}
		arr, _ := v.([]any)
		var filtered []string
		for _, r := range arr {
			s := strings.ToLower(fmt.Sprint(r))
			if q == "" || strings.HasPrefix(s, q) {
				filtered = append(filtered, fmt.Sprint(r))
			}
		}
		if len(filtered) == 0 {
			continue
		}
		suffix := ""
		if q != "" {
			suffix = fmt.Sprintf(" matching %q", route)
		}
		lines = append(lines, fmt.Sprintf("%s%s (%d): %s", reg, suffix, len(filtered), strings.Join(filtered, ", ")))
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No GMB routes found%s. Regions: HKI, KLN, NT.", routeSuffix(route)), nil
	}
	return strings.Join(lines, "\n"), nil
}

func routeSuffix(route string) string {
	if route == "" {
		return ""
	}
	return fmt.Sprintf(" for %q", route)
}

func RouteVariants(ctx context.Context, region, routeCode string, lang Lang) (string, error) {
	d, err := gmbGet(ctx, fmt.Sprintf("/route/%s/%s", region, routeCode), TTLStaticRoute)
	if err != nil {
		return "", err
	}
	rows := AsObjList(Arr(d, "data"))
	if len(rows) == 0 {
		return fmt.Sprintf("No GMB route %s in region %s. Regions: HKI, KLN, NT.", routeCode, region), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "GMB %s route %s variants:", region, routeCode)
	for _, v := range rows {
		fmt.Fprintf(&b, "\n- route_id %s [%s]", numStr(v["route_id"]), Pick(v, "description", lang))
		for _, dir := range AsObjList(Arr(v, "directions")) {
			remarks := Pick(dir, "remarks", lang)
			if remarks == "" {
				fmt.Fprintf(&b, "\n    route_seq %v: %s → %s", dir["route_seq"], Pick(dir, "orig", lang), Pick(dir, "dest", lang))
			} else {
				fmt.Fprintf(&b, "\n    route_seq %v: %s → %s (%s)", dir["route_seq"], Pick(dir, "orig", lang), Pick(dir, "dest", lang), remarks)
			}
		}
	}
	return b.String(), nil
}

func GmbRouteStops(ctx context.Context, routeID, routeSeq string, lang Lang) (string, error) {
	d, err := gmbGet(ctx, fmt.Sprintf("/route-stop/%s/%s", routeID, routeSeq), TTLStaticRoute)
	if err != nil {
		return "", err
	}
	stops := AsObjList(Arr(Obj(d, "data"), "route_stops"))
	if len(stops) == 0 {
		return fmt.Sprintf("No stops for GMB route_id %s route_seq %s.", routeID, routeSeq), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "GMB route_id %s (route_seq %s) — %d stops:", routeID, routeSeq, len(stops))
	for _, s := range stops {
		fmt.Fprintf(&b, "\n%s. %s (stop_id %s)", numStr(s["stop_seq"]), Pick(s, "name", lang), numStr(s["stop_id"]))
	}
	return b.String(), nil
}

func GmbEta(ctx context.Context, routeID, stopID string, lang Lang) (string, error) {
	d, err := gmbGet(ctx, fmt.Sprintf("/eta/route-stop/%s/%s", routeID, stopID), TTLEta)
	if err != nil {
		return "", err
	}
	etas := []map[string]any{}
	for _, r := range AsObjList(Arr(d, "data")) {
		if enabled, ok := r["enabled"].(bool); ok && !enabled {
			continue
		}
		etas = append(etas, AsObjList(Arr(r, "eta"))...)
	}
	if len(etas) == 0 {
		return fmt.Sprintf("No GMB ETA data for route_id %s stop_id %s. Get valid ids via get_gmb_route_list → get_gmb_route_variants → get_gmb_route_stops.", routeID, stopID), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "GMB stop %s on route_id %s ETAs:", stopID, routeID)
	for _, e := range etas {
		t := "-"
		if ts := Str(e, "timestamp"); len(ts) >= 16 {
			t = ts[11:16]
		}
		diff := ""
		if f, ok := e["diff"].(float64); ok {
			diff = fmt.Sprintf(" (%v min)", int(f))
		}
		remark := Pick(e, "remark", lang)
		if remark != "" {
			remark = " " + remark
		}
		fmt.Fprintf(&b, "\n- %s%s%s", t, diff, remark)
	}
	return b.String(), nil
}
