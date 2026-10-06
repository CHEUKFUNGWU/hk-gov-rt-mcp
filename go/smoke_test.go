// Smoke test: connect an MCP client to the server over in-memory transport
// and call every tool against the live upstream APIs.
// Run: go test -v -timeout 6m ./...
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type call struct {
	tool string
	args map[string]any
}

var calls = []call{
	// weather
	{"get_current_weather", map[string]any{"lang": "tc"}},
	{"get_local_forecast", map[string]any{"lang": "tc"}},
	{"get_9day_forecast", map[string]any{"lang": "en"}},
	{"get_weather_warnings", map[string]any{"lang": "tc"}},
	// KMB
	{"get_kmb_route_list", map[string]any{"route": "1A", "lang": "tc"}},
	{"get_kmb_route_stops", map[string]any{"route": "1A", "bound": "O", "service_type": "1", "lang": "tc"}},
	{"get_kmb_route_eta", map[string]any{"route": "1A", "service_type": "1", "lang": "tc"}},
	{"get_kmb_stop_eta", map[string]any{"stop_id": "184832", "lang": "tc"}},
	// CTB
	{"get_ctb_route_list", map[string]any{"route": "1", "lang": "tc"}},
	{"get_ctb_route_stops", map[string]any{"route": "1", "direction": "outbound", "lang": "tc"}},
	{"get_ctb_route_eta", map[string]any{"route": "1", "direction": "outbound", "lang": "tc"}},
	{"get_ctb_stop_eta", map[string]any{"route": "1", "stop_id": "001027", "lang": "tc"}},
	// MTR
	{"get_mtr_station_codes", map[string]any{"query": "中環", "lang": "tc"}},
		{"get_mtr_frequency", map[string]any{"line": "TWL", "lang": "tc"}},
		{"get_mtr_frequency", map[string]any{"lang": "en"}},
	{"get_mtr_schedule", map[string]any{"line": "twl", "station": "cen", "lang": "tc"}},
	{"get_lrt_schedule", map[string]any{"station_id": "100", "lang": "tc"}},
	{"get_mtr_bus_schedule", map[string]any{"route": "K51", "lang": "tc"}},
	// GMB
	{"get_gmb_route_list", map[string]any{"region": "HKI", "route": "1", "lang": "tc"}},
	{"get_gmb_route_variants", map[string]any{"region": "HKI", "route_code": "1", "lang": "tc"}},
	{"get_gmb_route_stops", map[string]any{"route_id": "2006408", "route_seq": "1", "lang": "tc"}},
	{"get_gmb_eta", map[string]any{"route_id": "2006408", "stop_id": "20014489", "lang": "tc"}},
	// TD
	{"get_traffic_snapshot", map[string]any{"camera_ids": []string{"A01"}, "lang": "tc"}},
	{"get_traffic_speed", map[string]any{"lang": "tc"}},
	{"get_parking_vacancy", map[string]any{"keyword": "沙田", "lang": "tc"}},
}

func TestAllTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0.0.1"}, nil)
	server := newServer()
	ct, st := mcp.NewInMemoryTransports()
	// The in-memory transport blocks writes until the peer reads, so the
	// server side must be connected before the client sends "initialize".
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	list, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := make([]string, 0, len(list.Tools))
	for _, tl := range list.Tools {
		names = append(names, tl.Name)
	}
	t.Logf("server exposes %d tools: %s", len(list.Tools), strings.Join(names, ", "))
	unique := map[string]bool{}
	for _, c := range calls {
		unique[c.tool] = true
	}
	if len(list.Tools) != len(unique) {
		t.Errorf("tool count = %d, want %d", len(list.Tools), len(unique))
	}

	failed := 0
	for _, c := range calls {
		t.Run(c.tool, func(t *testing.T) {
			callCtx, callCancel := context.WithTimeout(ctx, 90*time.Second)
			defer callCancel()
			t0 := time.Now()
			res, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: c.tool, Arguments: c.args})
			ms := time.Since(t0).Milliseconds()
			if err != nil {
				failed++
				t.Errorf("ERROR %s: %v", c.tool, err)
				return
			}
			var text strings.Builder
			for _, b := range res.Content {
				if tc, ok := b.(*mcp.TextContent); ok {
					text.WriteString(tc.Text)
				}
			}
			out := strings.ReplaceAll(text.String(), "\n", " | ")
			if len(out) > 300 {
				out = out[:300]
			}
			if res.IsError {
				failed++
				t.Logf("FAIL %s (%dms): %s", c.tool, ms, out)
			} else {
				t.Logf("PASS %s (%dms): %s", c.tool, ms, out)
			}
		})
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d tool call(s) returned errors\n", failed)
		t.Errorf("%d tool call(s) returned errors", failed)
	}
}
