package hkapi

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const kmbBase = "https://data.etabus.gov.hk/v1/transport/kmb"

var boundWord = map[string]string{"O": "outbound", "I": "inbound"}

// BoundLabel renders O/I direction in the requested language.
var BoundLabel = map[Lang]map[string]string{
	TC: {"O": "去程", "I": "回程"},
	SC: {"O": "去程", "I": "回程"},
	EN: {"O": "outbound", "I": "inbound"},
}

func kmbGet(ctx context.Context, path string, ttl time.Duration) (map[string]any, error) {
	return Cached(ctx, "kmb:"+path, ttl, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, kmbBase+path, 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
}

type KmbRouteVariant struct {
	Co          string
	Route       string
	Bound       string
	ServiceType string
	Orig        string
	Dest        string
}

func FindRouteVariants(ctx context.Context, route string, lang Lang) ([]KmbRouteVariant, error) {
	all, err := kmbGet(ctx, "/route", TTLStaticRoute)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(route))
	var out []KmbRouteVariant
	for _, r := range AsObjList(Arr(all, "data")) {
		routeNo := Str(r, "route")
		if !strings.HasPrefix(strings.ToLower(routeNo), q) {
			continue
		}
		out = append(out, KmbRouteVariant{
			Co:          Str(r, "co"),
			Route:       routeNo,
			Bound:       Str(r, "bound"),
			ServiceType: Str(r, "service_type"),
			Orig:        Pick(r, "orig", lang),
			Dest:        Pick(r, "dest", lang),
		})
	}
	return out, nil
}

func kmbStopName(ctx context.Context, stopID string, lang Lang) string {
	s, err := Cached(ctx, "kmb:stop:"+stopID, TTLStaticRoute, func() (map[string]any, error) {
		var out map[string]any
		if err := GetJSON(ctx, fmt.Sprintf("%s/stop/%s", kmbBase, stopID), 10*time.Second, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return stopID
	}
	return Pick(Obj(s, "data"), "name", lang)
}

func RouteStops(ctx context.Context, route, bound, serviceType string, lang Lang) (string, error) {
	rs, err := kmbGet(ctx, fmt.Sprintf("/route-stop/%s/%s/%s", route, boundWord[bound], serviceType), TTLStaticRoute)
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
			names[i] = kmbStopName(ctx, stopID, lang)
		}(i, Str(r, "stop"))
	}
	wg.Wait()
	var b strings.Builder
	fmt.Fprintf(&b, "KMB route %s %s service_type %s — %d stops:\n", route, BoundLabel[lang][bound], serviceType, len(rows))
	for i, r := range rows {
		fmt.Fprintf(&b, "%v. %s (stop %s)\n", r["seq"], names[i], Str(r, "stop"))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func fmtEtas(etas []map[string]any, lang Lang) string {
	if len(etas) == 0 {
		if lang == EN {
			return "no ETA"
		}
		return "暫無到站時間"
	}
	parts := []string{}
	for _, e := range etas {
		t := "-"
		if eta := Str(e, "eta"); len(eta) >= 16 {
			t = eta[11:16]
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", t, Pick(e, "rmk", lang)))
	}
	return strings.Join(parts, ", ")
}

func RouteEta(ctx context.Context, route, serviceType string, lang Lang) (string, error) {
	eta, err := kmbGet(ctx, fmt.Sprintf("/route-eta/%s/%s", route, serviceType), TTLEta)
	if err != nil {
		return "", err
	}
	rows := AsObjList(Arr(eta, "data"))
	if len(rows) == 0 {
		return fmt.Sprintf("No ETA data for KMB route %s (service_type %s).", route, serviceType), nil
	}
	bounds := []string{}
	seen := map[string]bool{}
	for _, r := range rows {
		d := Str(r, "dir")
		if !seen[d] {
			seen[d] = true
			bounds = append(bounds, d)
		}
	}
	stopNames := map[string]string{}
	for _, b := range bounds {
		rs, err := kmbGet(ctx, fmt.Sprintf("/route-stop/%s/%s/%s", route, boundWord[b], serviceType), TTLStaticRoute)
		if err != nil {
			continue
		}
		for _, r := range AsObjList(Arr(rs, "data")) {
			stopNames[fmt.Sprintf("%s:%v", b, r["seq"])] = Str(r, "stop")
		}
	}
	type key struct {
		b   string
		seq int
	}
	groups := map[key][]map[string]any{}
	seqs := map[string][]int{}
	for _, r := range rows {
		b := Str(r, "dir")
		seq, _ := strconv.Atoi(fmt.Sprintf("%v", r["seq"]))
		groups[key{b, seq}] = append(groups[key{b, seq}], r)
		found := false
		for _, s2 := range seqs[b] {
			if s2 == seq {
				found = true
				break
			}
		}
		if !found {
			seqs[b] = append(seqs[b], seq)
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "KMB route %s ETAs (generated %s)", route, Str(eta, "generated_timestamp"))
	for _, b := range bounds {
		fmt.Fprintf(&out, "\n-- %s --", BoundLabel[lang][b])
		sl := seqs[b]
		sort.Ints(sl)
		for _, seq := range sl {
			group := groups[key{b, seq}]
			dest := ""
			if len(group) > 0 {
				dest = Pick(group[0], "dest", lang)
			}
			name := fmt.Sprintf("seq %d", seq)
			if sid, ok := stopNames[fmt.Sprintf("%s:%d", b, seq)]; ok {
				name = sid
			}
			fmt.Fprintf(&out, "\n%d. %s → %s: %s", seq, name, dest, fmtEtas(group, lang))
		}
	}
	if vs, err := FindRouteVariants(ctx, route, lang); err == nil && len(vs) > 0 {
		fmt.Fprintf(&out, "\n(route runs %s ↔ %s)", vs[0].Orig, vs[0].Dest)
	}
	return out.String(), nil
}

func StopEta(ctx context.Context, stopID string, lang Lang) (string, error) {
	eta, err := kmbGet(ctx, "/stop-eta/"+stopID, TTLEta)
	if err != nil {
		return "", err
	}
	rows := AsObjList(Arr(eta, "data"))
	name := kmbStopName(ctx, stopID, lang)
	if len(rows) == 0 {
		return fmt.Sprintf("No ETA data for KMB stop %s.", stopID), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "KMB stop %s (%s) ETAs:", name, stopID)
	for _, r := range rows {
		dir := Str(r, "dir")
		fmt.Fprintf(&b, "\n- %s %s → %s: %s", Str(r, "route"), BoundLabel[lang][dir], Pick(r, "dest", lang), fmtEtas([]map[string]any{r}, lang))
	}
	return b.String(), nil
}
