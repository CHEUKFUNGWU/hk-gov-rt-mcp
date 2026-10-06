package hkapi

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const cidURL = "https://rt.data.gov.hk/v2/transport/cid/camera"

// CameraSnapshots queries the TD CID API for live snapshot URLs. The upstream
// has been observed to 403 some networks; failures are reported verbatim.
func CameraSnapshots(ctx context.Context, cameraIDs []string, locale Lang) (string, error) {
	payload := map[string]any{"locale": string(locale), "cameraIdList": cameraIDs}
	d, err := Cached(ctx, fmt.Sprintf("cid:%v:%s", cameraIDs, locale), TTLTraffic, func() (map[string]any, error) {
		var out map[string]any
		if err := PostJSON(ctx, cidURL, payload, &out); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return fmt.Sprintf("Traffic snapshot API unavailable (%s). Official endpoint: POST %s with body {\"locale\":\"tc\",\"cameraIdList\":[\"A01\",...]}. TD camera id prefixes: A (HK Island), KE (Kowloon East), KW, KK, LCK, ST, TW, TY, NT (New Territories). Retry later or check the data.gov.hk dataset「交通快拍圖像」.", err.Error(), cidURL), nil
	}
	var list []any
	for _, k := range []string{"results", "cameraIdList", "data"} {
		if arr, ok := d[k].([]any); ok && len(arr) > 0 {
			list = arr
			break
		}
	}
	if len(list) == 0 {
		raw, _ := jsonMarshal(d)
		return fmt.Sprintf("Camera API returned no entries for ids %v. Raw: %.500s", cameraIDs, raw), nil
	}
	var b strings.Builder
	b.WriteString("Traffic camera snapshots:")
	for _, it := range list {
		c, _ := it.(map[string]any)
		if c == nil {
			continue
		}
		id := "?"
		if v, ok := c["cameraId"]; ok {
			id = fmt.Sprint(v)
		} else if v, ok := c["camera_id"]; ok {
			id = fmt.Sprint(v)
		}
		road := Str(c, "roadName")
		if road == "" {
			road = Str(c, "road_name")
		}
		u := Str(c, "url")
		if u == "" {
			u = Str(c, "snapshotUrl")
		}
		if u == "" {
			raw, _ := jsonMarshal(c)
			fmt.Fprintf(&b, "\n- camera %s: %.300s", id, raw)
			continue
		}
		if road != "" {
			fmt.Fprintf(&b, "\n- camera %s [%s]: %s", id, road, u)
		} else {
			fmt.Fprintf(&b, "\n- camera %s: %s", id, u)
		}
	}
	return b.String(), nil
}

type speedRow struct {
	LINK_ID             string
	CAPTURE_DATE        string
	TRAFFIC_SPEED       string
	ROAD_SATURATION_LVL string
}

var speedmapAttr = regexp.MustCompile(`(\w+)="([^"]*)"`)

// TrafficSpeed parses the TD speed map XML. The feed has been returning 503
// lately; failures are reported verbatim.
func TrafficSpeed(ctx context.Context) (string, error) {
	const url = "https://resource.data.one.gov.hk/td/speedmap.xml"
	xml, err := Cached(ctx, "td:speedmap", TTLTraffic, func() (string, error) {
		return GetText(ctx, url, 15*time.Second)
	})
	if err != nil {
		return fmt.Sprintf("Traffic speed feed unavailable (%s). Official endpoint: %s — TD has been returning 503 for this feed; retry later.", err.Error(), url), nil
	}
	re := regexp.MustCompile(`<speedmap\s+([^/>]+)/>`)
	rows := []speedRow{}
	for _, m := range re.FindAllStringSubmatch(xml, -1) {
		row := speedRow{}
		for _, a := range speedmapAttr.FindAllStringSubmatch(m[1], -1) {
			switch a[1] {
			case "LINK_ID":
				row.LINK_ID = a[2]
			case "CAPTURE_DATE":
				row.CAPTURE_DATE = a[2]
			case "TRAFFIC_SPEED":
				row.TRAFFIC_SPEED = a[2]
			case "ROAD_SATURATION_LEVEL":
				row.ROAD_SATURATION_LVL = a[2]
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return fmt.Sprintf("Speed map returned no rows. Raw start: %.200s", xml), nil
	}
	counts := map[string]int{}
	var order []string
	for _, r := range rows {
		if _, ok := counts[r.ROAD_SATURATION_LVL]; !ok {
			order = append(order, r.ROAD_SATURATION_LVL)
		}
		counts[r.ROAD_SATURATION_LVL]++
	}
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "TD speed map — %d road links (%s, captured %s).\nFirst 30 links:\n", len(rows), strings.Join(parts, ", "), rows[0].CAPTURE_DATE)
	n := len(rows)
	if n > 30 {
		n = 30
	}
	for _, r := range rows[:n] {
		fmt.Fprintf(&b, "- link %s: %s km/h [%s] @ %s\n", r.LINK_ID, r.TRAFFIC_SPEED, r.ROAD_SATURATION_LVL, r.CAPTURE_DATE)
	}
	fmt.Fprintf(&b, "(Full data: %d links; LINK_ID geometry is published in the TD speed map dataspec.)", len(rows))
	return b.String(), nil
}
