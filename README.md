[English](README.md) | [繁體中文（香港）](README.zh-HK.md) | [简体中文](README.zh-CN.md)

# hk-gov-rt-mcp

An MCP (Model Context Protocol) server wrapping Hong Kong government real-time open data APIs from the weather and transport categories on data.gov.hk.

**Dual implementations** — one shared tool contract, two languages, identical tools, parameters and output format:

| Implementation | Location | Tech | Run |
|---|---|---|---|
| TypeScript | `ts/` | Official `@modelcontextprotocol/sdk` ^1.32 + zod + Node ≥20 | `node ts/dist/index.js` |
| Go | `go/` | Official `github.com/modelcontextprotocol/go-sdk` v1.8.0 | `go/bin/hk-gov-mcp` (single static binary) |

Both support the **stdio** (default) and **Streamable HTTP** (stateless) transports. The initialize handshake negotiates protocol revision **2025-11-25** — the newest revision supported by the current official SDKs (the spec site's latest documentation is 2026-07-28). Every tool carries a `readOnlyHint` annotation, and the server provides top-level `instructions`.

## Tools (24)

All tools accept an optional `lang` parameter: `tc` (Traditional Chinese, default) / `sc` (Simplified Chinese) / `en` (English).

### Weather — Hong Kong Observatory (data.weather.gov.hk)

| Tool | Description |
|---|---|
| `get_current_weather` | Current weather report: temperatures at ~27 stations, humidity, rainfall by 18 districts, UV index, weather icon and any warning messages |
| `get_local_forecast` | Local forecast: general situation, tropical cyclone info, today/tonight/tomorrow description, outlook |
| `get_9day_forecast` | 9-day forecast: daily weather / min-max temperature / humidity range / wind / probability of significant rain, plus sea temperature |
| `get_weather_warnings` | Warnings currently in force (summary + details) and special weather tips; replies explicitly when none are active |

### KMB / Long Win (data.etabus.gov.hk)

| Tool | Description |
|---|---|
| `get_kmb_route_list` · `route` | Route lookup (prefix match): each variant's direction, origin, destination and service_type |
| `get_kmb_route_stops` · `route, bound(O/I), service_type?` | Ordered stop list with stop ids |
| `get_kmb_route_eta` · `route, service_type?` | Live ETAs for every stop (both directions, with remarks) |
| `get_kmb_stop_eta` · `stop_id` | Live ETAs of all routes serving one stop |

### Citybus CTB (rt.data.gov.hk/v2/transport/citybus)

| Tool | Description |
|---|---|
| `get_ctb_route_list` · `route` | Route lookup with origin/destination |
| `get_ctb_route_stops` · `route, direction(outbound/inbound)` | Ordered stop list |
| `get_ctb_route_eta` · `route, direction` | Stop-by-stop live ETAs (assembled from the per-stop ETA API) |
| `get_ctb_stop_eta` · `route, stop_id` | Live ETAs of one stop for one route |

### MTR (rt.data.gov.hk/v1/transport/mtr)

| Tool | Description |
|---|---|
| `get_mtr_schedule` · `line, station` | Heavy rail live schedule: next trains per platform/direction with destination, minutes-to-train (ttnt) and service-delay flag |
| `get_mtr_station_codes` · `query` | Find line/station codes by station name (any language) or code — embedded 10-line / 120-station table |
| `get_mtr_frequency` · `line?` | Published average headways (weekday AM/PM peak, off-peak, Sat, Sun & holidays) for all lines/segments and Light Rail routes — static snapshot scraped from mtr.com.hk (`scripts/scrape_mtr_frequencies.py`), answers "how often / still running around <time>" questions; NOT real-time |
| `get_lrt_schedule` · `station_id` | Light Rail live schedule — embedded 68-stop id table (e.g. 100 = Siu Hong) |
| `get_mtr_bus_schedule` · `route` | MTR Bus live schedule (upstream unstable at the moment; reported gracefully) |

### Green Minibus GMB (data.etagmb.gov.hk)

| Tool | Description |
|---|---|
| `get_gmb_route_list` · `region?(HKI/KLN/NT), route?` | Route numbers |
| `get_gmb_route_variants` · `region, route_code` | route_id variants with directions (route_seq) |
| `get_gmb_route_stops` · `route_id, route_seq` | Ordered stop list with stop_ids |
| `get_gmb_eta` · `route_id, stop_id` | Live ETA |

### Transport Department TD

| Tool | Description |
|---|---|
| `get_traffic_snapshot` · `camera_ids?` | Live traffic camera snapshot image URLs (CID API) |
| `get_traffic_speed` | Live average speed / saturation level (GOOD/AVERAGE/BAD) for major road links (speed map XML) |
| `get_parking_vacancy` · `keyword?` | Live vacant-space counts at TD participating car parks (filter by name / district) |

> **Known upstream issues** — `get_mtr_bus_schedule` (404), `get_traffic_snapshot` (403) and `get_traffic_speed` (503) were broken or access-restricted upstream at development time (2026-10). The tools report this gracefully (readable message + official endpoint) instead of failing; they self-heal once the feeds recover. The other 21 tools are fully operational and verified.

## Typical flows

- **Bus ETA**: `get_kmb_route_list(1A)` → `get_kmb_route_stops(1A, O, 1)` → `get_kmb_route_eta(1A, 1)` or `get_kmb_stop_eta(stop_id)`
- **MTR**: `get_mtr_station_codes("Central")` → `get_mtr_schedule(twl, cen)`
- **GMB**: `get_gmb_route_list(HKI, 1)` → `get_gmb_route_variants(HKI, "1")` → `get_gmb_route_stops(route_id, route_seq)` → `get_gmb_eta(route_id, stop_id)`
- **Parking**: `get_parking_vacancy("Sha Tin")`

## Wiring it up

### ZCode / Claude Desktop — TypeScript

```json
{
  "mcpServers": {
    "hk-gov": {
      "command": "node",
      "args": ["/absolute/path/to/hk-gov-rt-mcp/ts/dist/index.js"]
    }
  }
}
```

### ZCode / Claude Desktop — Go

```bash
cd go && go build -o bin/hk-gov-mcp .
```

```json
{
  "mcpServers": {
    "hk-gov-go": {
      "command": "/absolute/path/to/hk-gov-rt-mcp/go/bin/hk-gov-mcp"
    }
  }
}
```

### Streamable HTTP mode (remote / multiple clients)

```bash
# TypeScript
node ts/dist/index.js --http --port 8819
# Go
go/bin/hk-gov-mcp -http 127.0.0.1:8819
```

Then point the client at the URL:

```json
{ "mcpServers": { "hk-gov": { "url": "http://127.0.0.1:8819/mcp" } } }
```

## Development & tests

```bash
# TypeScript
cd ts && npm install && npm run build && npm run smoke
npx @modelcontextprotocol/inspector node dist/index.js   # manual inspection

# Go
cd go && go build -o bin/hk-gov-mcp . && go test -v -timeout 12m .
```

The smoke tests connect an MCP client over in-memory transport and call all 24 tools against the live upstream APIs.

## Caching policy (identical in both implementations)

| Data | TTL |
|---|---|
| Live ETAs | 20s |
| Current weather / warnings | 60s |
| Forecasts | 10 min |
| Speed map | 2 min |
| Car park vacancy | 60s |
| Route / stop static data | 6h |

Upstream timeouts are 10s (30s for the large Citybus route list); failures return a readable `isError: true` message so the MCP connection never breaks. Failed fetches are cached for 30s to avoid hammering a failing upstream.

## Data sources

- HKO: `data.weather.gov.hk/weatherAPI/opendata` (rhrread / flw / fnd / warnsum / warningInfo / swt)
- KMB: `data.etabus.gov.hk` (v1 open API)
- Citybus / MTR / Light Rail / MTR Bus / CID: `rt.data.gov.hk`
- Green minibus: `data.etagmb.gov.hk`
- Transport Department: `resource.data.one.gov.hk/td/*` (speed map, car parks), `api.data.gov.hk/v1/carpark-info-vacancy`
- MTR published frequencies: scraped from `mtr.com.hk` train service page into the embedded `mtr-frequencies.json` (snapshot date inside; refresh with `python3 scripts/scrape_mtr_frequencies.py`)
- MTR station code tables: generated from `opendata.mtr.com.hk` CSVs (`mtr_lines_and_stations.csv`, `light_rail_routes_and_stops.csv`) into the embedded JSON under `ts/src/transport/data/` and `go/internal/hkapi/data/`
