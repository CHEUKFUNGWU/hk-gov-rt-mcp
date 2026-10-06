// Package tools registers all hk-gov-rt-mcp tools on an MCP server. Tool
// names, parameters and output formatting mirror the TypeScript
// implementation in ../ts.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"hk-gov-rt-mcp/go/internal/hkapi"
)

type langArgs struct {
	Lang string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
}

func normalize(argsLang string) hkapi.Lang {
	return hkapi.NormalizeLang(argsLang)
}

// call runs a tool body, converting upstream errors into isError results so a
// failing API never breaks the MCP connection.
func call(fn func(ctx context.Context) (string, error)) (*mcp.CallToolResult, any, error) {
	ctx := context.Background()
	text, err := fn(ctx)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + err.Error()}},
		}, nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}

func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPtr(true)}
}

func boolPtr(b bool) *bool { return &b }

const langDesc = "Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"

// Register wires every tool onto the server (23 tools, mirroring ../ts).
func Register(server *mcp.Server) {
	/* ---- weather ---- */
	type currentWeatherArgs struct {
		Lang string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_current_weather",
		Description: "Hong Kong Observatory current weather report: temperatures at ~27 stations, humidity, rainfall by district, UV index, weather icon and any warning messages. Updates every ~10 minutes.",
		Annotations: readOnly("HKO current weather report"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a currentWeatherArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) { return hkapi.FormatCurrent(ctx, normalize(a.Lang)) })
	})

	type noLangArgs struct{}
	_ = noLangArgs{}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_local_forecast",
		Description: "Hong Kong Observatory local forecast: general situation, tropical cyclone info, today/tonight/tomorrow forecast description and outlook. Updates roughly hourly.",
		Annotations: readOnly("HKO local weather forecast"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a langArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) { return hkapi.FormatLocal(ctx, normalize(a.Lang)) })
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_9day_forecast",
		Description: "Hong Kong Observatory 9-day weather forecast: daily weather, min/max temperature, humidity range, wind, probability of significant rain (PSR), plus sea and soil temperatures.",
		Annotations: readOnly("HKO 9-day forecast"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a langArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) { return hkapi.FormatNineDay(ctx, normalize(a.Lang)) })
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_weather_warnings",
		Description: "Weather warnings currently in force (summary + details) and HKO special weather tips. Returns an explicit 'no warnings' message when none are active.",
		Annotations: readOnly("HKO weather warnings"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a langArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) { return hkapi.FormatWarnings(ctx, normalize(a.Lang)) })
	})

	/* ---- KMB ---- */
	type kmbRouteListArgs struct {
		Route string `json:"route" jsonschema:"Route number e.g. 1A / 269D / A10"`
		Lang  string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_kmb_route_list",
		Description: "Look up KMB / Long Win bus routes by route number (prefix match) and get each variant's direction, origin, destination and service_type. route is required to keep the response small.",
		Annotations: readOnly("KMB route lookup"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a kmbRouteListArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			lang := normalize(a.Lang)
			vs, err := hkapi.FindRouteVariants(ctx, a.Route, lang)
			if err != nil {
				return "", err
			}
			if len(vs) == 0 {
				return fmt.Sprintf("No KMB route matching %q.", a.Route), nil
			}
			lines := make([]string, 0, len(vs))
			for _, v := range vs {
				label := hkapi.BoundLabel[lang][v.Bound]
				if label == "" {
					label = v.Bound
				}
				lines = append(lines, fmt.Sprintf("%s [%s] service_type %s: %s → %s", v.Route, label, v.ServiceType, v.Orig, v.Dest))
			}
			return strings.Join(lines, "\n"), nil
		})
	})

	type kmbRouteStopsArgs struct {
		Route       string `json:"route" jsonschema:"Route number e.g. 1A / 269D / A10"`
		Bound       string `json:"bound" jsonschema:"Direction — O 去程 outbound / I 回程 inbound,enum=O,I"`
		ServiceType string `json:"service_type,omitempty" jsonschema:"KMB service type variant (normally leave as-is),default=1"`
		Lang        string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_kmb_route_stops",
		Description: "Ordered stop list of a KMB route variant (from get_kmb_route_list): seq, stop name and stop id.",
		Annotations: readOnly("KMB route stops"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a kmbRouteStopsArgs) (*mcp.CallToolResult, any, error) {
		st := a.ServiceType
		if st == "" {
			st = "1"
		}
		return call(func(ctx context.Context) (string, error) {
			return hkapi.RouteStops(ctx, a.Route, a.Bound, st, normalize(a.Lang))
		})
	})

	type kmbRouteEtaArgs struct {
		Route       string `json:"route" jsonschema:"Route number e.g. 1A / 269D / A10"`
		ServiceType string `json:"service_type,omitempty" jsonschema:"KMB service type variant (normally leave as-is),default=1"`
		Lang        string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_kmb_route_eta",
		Description: "Real-time arrival times for every stop of a KMB route (both directions), with destination and remark. Updates every minute.",
		Annotations: readOnly("KMB route ETAs"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a kmbRouteEtaArgs) (*mcp.CallToolResult, any, error) {
		st := a.ServiceType
		if st == "" {
			st = "1"
		}
		return call(func(ctx context.Context) (string, error) {
			return hkapi.RouteEta(ctx, a.Route, st, normalize(a.Lang))
		})
	})

	type kmbStopEtaArgs struct {
		StopID string `json:"stop_id" jsonschema:"KMB stop id e.g. 184832"`
		Lang   string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_kmb_stop_eta",
		Description: "Real-time ETAs of all routes serving a KMB stop. Get stop ids from get_kmb_route_stops.",
		Annotations: readOnly("KMB stop ETA"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a kmbStopEtaArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.StopEta(ctx, a.StopID, normalize(a.Lang))
		})
	})

	/* ---- Citybus ---- */
	type ctbRouteListArgs struct {
		Route string `json:"route" jsonschema:"Route number e.g. 1 / 15 / A10"`
		Lang  string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ctb_route_list",
		Description: "Look up Citybus (CTB) routes by route number (prefix match) with origin/destination. route is required to keep the response small.",
		Annotations: readOnly("Citybus route lookup"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a ctbRouteListArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			lang := normalize(a.Lang)
			rs, err := hkapi.FindRoutes(ctx, a.Route)
			if err != nil {
				return "", err
			}
			if len(rs) == 0 {
				return fmt.Sprintf("No CTB route matching %q.", a.Route), nil
			}
			lines := make([]string, 0, len(rs))
			for _, r := range rs {
				orig := hkapi.Str(r, "orig_"+string(lang))
				if orig == "" {
					orig = hkapi.Str(r, "orig_en")
				}
				dest := hkapi.Str(r, "dest_"+string(lang))
				if dest == "" {
					dest = hkapi.Str(r, "dest_en")
				}
				lines = append(lines, fmt.Sprintf("%s: %s → %s", hkapi.Str(r, "route"), orig, dest))
			}
			return strings.Join(lines, "\n"), nil
		})
	})

	type ctbDirectionArgs struct {
		Route     string `json:"route" jsonschema:"Route number e.g. 1 / 15 / A10"`
		Direction string `json:"direction" jsonschema:"Direction — outbound 去程 / inbound 回程,enum=outbound,inbound"`
		Lang      string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ctb_route_stops",
		Description: "Ordered stop list of a Citybus route direction (inbound/outbound): seq, stop name and stop id.",
		Annotations: readOnly("Citybus route stops"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a ctbDirectionArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.CtbRouteStops(ctx, a.Route, a.Direction, normalize(a.Lang))
		})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ctb_route_eta",
		Description: "Real-time ETAs for every stop of a Citybus route direction (assembled stop-by-stop from the CTB per-stop ETA API).",
		Annotations: readOnly("Citybus route ETAs"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a ctbDirectionArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.CtbRouteEta(ctx, a.Route, a.Direction, normalize(a.Lang))
		})
	})

	type ctbStopEtaArgs struct {
		Route  string `json:"route" jsonschema:"Route number"`
		StopID string `json:"stop_id" jsonschema:"Citybus stop id, e.g. 001027"`
		Lang   string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ctb_stop_eta",
		Description: "Real-time ETAs of a Citybus stop for one route. Get stop ids from get_ctb_route_stops.",
		Annotations: readOnly("Citybus stop ETA"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a ctbStopEtaArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.CtbStopEta(ctx, a.Route, a.StopID, normalize(a.Lang))
		})
	})

	/* ---- MTR ---- */
	type mtrScheduleArgs struct {
		Line    string `json:"line" jsonschema:"MTR line code e.g. twl (Tsuen Wan) / eal / ktl / isl / tcl / tml / sil / ael / tkl / drl"`
		Station string `json:"station" jsonschema:"Station code on that line e.g. cen (Central) — comma-separated codes allowed"`
		Lang    string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mtr_schedule",
		Description: "Real-time MTR heavy rail train schedule for one line+station: next trains per platform/direction with destination, minutes-to-train (ttnt) and service-delay flag. Use get_mtr_station_codes to find line/station codes.",
		Annotations: readOnly("MTR train schedule"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a mtrScheduleArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.TrainSchedule(ctx, a.Line, a.Station, normalize(a.Lang))
		})
	})

	type mtrCodesArgs struct {
		Query string `json:"query" jsonschema:"Station name fragment (any language) or station code"`
		Lang  string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mtr_station_codes",
		Description: "Find MTR line and station codes by (partial) station name or exact code — needed before calling get_mtr_schedule.",
		Annotations: readOnly("MTR station code lookup"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a mtrCodesArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.LookupStationCodes(ctx, a.Query, normalize(a.Lang))
		})
	})

	type lrtScheduleArgs struct {
		StationID string `json:"station_id" jsonschema:"LRT numeric stop id e.g. 100"`
		Lang      string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_lrt_schedule",
		Description: "Real-time MTR Light Rail schedule for one stop: routes, destinations and arrival times per platform. Get stop ids from the embedded table (error messages list sample ids).",
		Annotations: readOnly("Light Rail schedule"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a lrtScheduleArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.LrtSchedule(ctx, a.StationID, normalize(a.Lang))
		})
	})

	type mtrBusArgs struct {
		Route string `json:"route" jsonschema:"MTR bus route id e.g. K51"`
		Lang  string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mtr_bus_schedule",
		Description: "MTR Bus / feeder bus real-time schedule by route id (e.g. K51). Upstream has been unstable; unavailability is reported gracefully.",
		Annotations: readOnly("MTR bus schedule"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a mtrBusArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) { return hkapi.BusSchedule(ctx, a.Route) })
	})

	/* ---- GMB ---- */
	type gmbRouteListArgs struct {
		Region string `json:"region,omitempty" jsonschema:"Region — HKI 香港島 / KLN 九龍 / NT 新界,enum=HKI,KLN,NT"`
		Route  string `json:"route,omitempty" jsonschema:"Route number prefix filter"`
		Lang   string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_gmb_route_list",
		Description: "List green minibus (專線小巴) route numbers, optionally filtered by region (HKI/KLN/NT) and route-number prefix. Use get_gmb_route_variants next to obtain route_id.",
		Annotations: readOnly("GMB route list"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a gmbRouteListArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.RouteList(ctx, a.Region, a.Route)
		})
	})

	type gmbVariantsArgs struct {
		Region    string `json:"region" jsonschema:"Region — HKI 香港島 / KLN 九龍 / NT 新界,enum=HKI,KLN,NT"`
		RouteCode string `json:"route_code" jsonschema:"GMB route number e.g. 1"`
		Lang      string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_gmb_route_variants",
		Description: "Get route_id variants (with directions and route_seq) for a green minibus route number — needed before get_gmb_route_stops.",
		Annotations: readOnly("GMB route variants"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a gmbVariantsArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.RouteVariants(ctx, a.Region, a.RouteCode, normalize(a.Lang))
		})
	})

	type gmbStopsArgs struct {
		RouteID  string `json:"route_id" jsonschema:"GMB numeric route_id e.g. 2006408"`
		RouteSeq string `json:"route_seq" jsonschema:"Direction from variants,enum=1,2"`
		Lang     string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_gmb_route_stops",
		Description: "Ordered stop list of a green minibus route (route_id + route_seq from get_gmb_route_variants).",
		Annotations: readOnly("GMB route stops"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a gmbStopsArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.GmbRouteStops(ctx, a.RouteID, a.RouteSeq, normalize(a.Lang))
		})
	})

	type gmbEtaArgs struct {
		RouteID string `json:"route_id" jsonschema:"GMB numeric route_id"`
		StopID  string `json:"stop_id" jsonschema:"GMB numeric stop_id"`
		Lang    string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_gmb_eta",
		Description: "Real-time ETA of a green minibus stop (route_id + stop_id from get_gmb_route_stops).",
		Annotations: readOnly("GMB stop ETA"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a gmbEtaArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.GmbEta(ctx, a.RouteID, a.StopID, normalize(a.Lang))
		})
	})

	/* ---- Transport Department ---- */
	type cameraArgs struct {
		CameraIDs []string `json:"camera_ids,omitempty" jsonschema:"Camera ids e.g. A01 / KE1"`
		Lang      string   `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_traffic_snapshot",
		Description: "Live traffic camera snapshot image URLs from Transport Department (CID API). Provide camera ids like A01, KE1, NT2.",
		Annotations: readOnly("TD traffic camera snapshots"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a cameraArgs) (*mcp.CallToolResult, any, error) {
		ids := a.CameraIDs
		if len(ids) == 0 {
			ids = []string{"A01"}
		}
		return call(func(ctx context.Context) (string, error) {
			return hkapi.CameraSnapshots(ctx, ids, normalize(a.Lang))
		})
	})

	type speedArgs struct {
		Lang string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_traffic_speed",
		Description: "Real-time average speed and saturation level (GOOD/AVERAGE/BAD) for major road links from the TD speed map feed. Upstream has been returning 503 lately; reported gracefully.",
		Annotations: readOnly("TD traffic speed map"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a speedArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) { return hkapi.TrafficSpeed(ctx) })
	})

	type parkingArgs struct {
		Keyword string `json:"keyword,omitempty" jsonschema:"Filter by car park name or district fragment"`
		Lang    string `json:"lang,omitempty" jsonschema:"Output language — tc 繁體中文 / sc 简体中文 / en English,default=tc"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_parking_vacancy",
		Description: "Live vacant-space counts for TD participating car parks (hourly category), searchable by car park name / district keyword.",
		Annotations: readOnly("TD car park vacancy"),
	}, func(ctx context.Context, req *mcp.CallToolRequest, a parkingArgs) (*mcp.CallToolResult, any, error) {
		return call(func(ctx context.Context) (string, error) {
			return hkapi.ParkingVacancy(ctx, a.Keyword, normalize(a.Lang))
		})
	})
}
