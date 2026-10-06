package hkapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const ctbBase = "https://rt.data.gov.hk/v2/transport/citybus"

func ctbGet(ctx context.Context, path string, ttl time.Duration) (map[string]any, error) {
	return Cached(ctx, "ctb:"+path, ttl, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, ctbBase+path, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
}

// FindRoutes filters the (large) CTB route list; the full list may be slow so
// it gets a longer fetch timeout and a long cache TTL.
func FindRoutes(ctx context.Context, route string) ([]map[string]any, error) {
	all, err := Cached(ctx, "ctb:/route/CTB", TTLStaticRoute, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, ctbBase+"/route/CTB", 30*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(route))
	var out []map[string]any
	for _, r := range AsObjList(Arr(all, "data")) {
		if strings.HasPrefix(strings.ToLower(Str(r, "route")), q) {
			out = append(out, r)
		}
	}
	return out, nil
}

func ctbStopName(ctx context.Context, stopID string, lang Lang) string {
	s, err := Cached(ctx, "ctb:stop:"+stopID, TTLStaticRoute, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, fmt.Sprintf("%s/stop/%s", ctbBase, stopID), 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return stopID
	}
	return Pick(Obj(s, "data"), "name", lang)
}

func CtbRouteStops(ctx context.Context, route, direction string, lang Lang) (string, error) {
	rs, err := ctbGet(ctx, fmt.Sprintf("/route-stop/CTB/%s/%s", route, direction), TTLStaticRoute)
	if err != nil {
		return "", err
	}
	rows := AsObjList(Arr(rs, "data"))
	names := make([]string, len(rows))
	var wg sync.WaitGroup
	for i, r := range rows {
		wg.Add(1)
		go func(i int, stopID string) {
			defer wg.Done()
			names[i] = ctbStopName(ctx, stopID, lang)
		}(i, Str(r, "stop"))
	}
	wg.Wait()
	var b strings.Builder
	fmt.Fprintf(&b, "CTB route %s %s — %d stops:\n", route, direction, len(rows))
	for i, r := range rows {
		fmt.Fprintf(&b, "%v. %s (stop %s)\n", r["seq"], names[i], Str(r, "stop"))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// ctbStopEtas fetches the per-stop ETA feed (CTB has no working route-wide
// ETA endpoint; /eta/CTB/{stop}/{route} is the documented one).
func ctbStopEtas(ctx context.Context, route, stopID string) []map[string]any {
	e, err := Cached(ctx, "ctb:/eta/CTB/"+stopID+"/"+route, TTLEta, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, fmt.Sprintf("%s/eta/CTB/%s/%s", ctbBase, stopID, route), 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil
	}
	return AsObjList(Arr(e, "data"))
}

func CtbRouteEta(ctx context.Context, route, direction string, lang Lang) (string, error) {
	rs, err := ctbGet(ctx, fmt.Sprintf("/route-stop/CTB/%s/%s", route, direction), TTLStaticRoute)
	if err != nil {
		return "", err
	}
	rows := AsObjList(Arr(rs, "data"))
	if len(rows) == 0 {
		return fmt.Sprintf("No stops found for CTB route %s (%s).", route, direction), nil
	}
	var wg sync.WaitGroup
	etas := make([][]map[string]any, len(rows))
	names := make([]string, len(rows))
	for i, r := range rows {
		wg.Add(2)
		go func(i int, stopID string) {
			defer wg.Done()
			etas[i] = ctbStopEtas(ctx, route, stopID)
		}(i, Str(r, "stop"))
		go func(i int, stopID string) {
			defer wg.Done()
			names[i] = ctbStopName(ctx, stopID, lang)
		}(i, Str(r, "stop"))
	}
	wg.Wait()
	var b strings.Builder
	fmt.Fprintf(&b, "CTB route %s %s ETAs (stop-by-stop):", route, direction)
	any := false
	for i, r := range rows {
		name := names[i]
		list := etas[i]
		if len(list) == 0 {
			if lang == EN {
				fmt.Fprintf(&b, "\n%v. %s: no ETA", r["seq"], name)
			} else {
				fmt.Fprintf(&b, "\n%v. %s: 暫無到站時間", r["seq"], name)
			}
			continue
		}
		any = true
		parts := []string{}
		for _, e := range list {
			t := "-"
			if eta := Str(e, "eta"); len(eta) >= 16 {
				t = eta[11:16]
			}
			parts = append(parts, fmt.Sprintf("%s (%s)", t, Pick(e, "rmk", lang)))
		}
		fmt.Fprintf(&b, "\n%v. %s: %s", r["seq"], name, strings.Join(parts, ", "))
	}
	if !any {
		if lang == EN {
			fmt.Fprint(&b, "\nNo ETA data available for this route right now.")
		} else {
			fmt.Fprint(&b, "\n現時此路線沒有到站時間資料。")
		}
	}
	return b.String(), nil
}

func CtbStopEta(ctx context.Context, route, stopID string, lang Lang) (string, error) {
	etas := ctbStopEtas(ctx, route, stopID)
	name := ctbStopName(ctx, stopID, lang)
	if len(etas) == 0 {
		return fmt.Sprintf("No ETA data for CTB stop %s (%s) on route %s.", name, stopID, route), nil
	}
	var b strings.Builder
	dir := Str(etas[0], "dir")
	label := dir
	if l, ok := BoundLabel[lang][dir]; ok {
		label = l
	}
	fmt.Fprintf(&b, "CTB stop %s (%s) route %s %s:", name, stopID, route, label)
	for _, e := range etas {
		t := "-"
		if eta := Str(e, "eta"); len(eta) >= 16 {
			t = eta[11:16]
		}
		fmt.Fprintf(&b, "\n- %s → %s (%s)", t, Pick(e, "dest", lang), Pick(e, "rmk", lang))
	}
	return b.String(), nil
}
