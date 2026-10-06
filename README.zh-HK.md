[English](README.md) | [繁體中文（香港）](README.zh-HK.md) | [简体中文](README.zh-CN.md)

# hk-gov-rt-mcp

將香港政府 data.gov.hk（氣象 + 運輸分類）的即時開放數據 API 包裝成 MCP（Model Context Protocol）伺服器。

**雙實作**：同一份工具契約，兩種語言實作，工具、參數與輸出格式完全一致：

| 實作 | 位置 | 技術 | 啟動 |
|---|---|---|---|
| TypeScript | `ts/` | 官方 `@modelcontextprotocol/sdk` ^1.32 + zod + Node ≥20 | `node ts/dist/index.js` |
| Go | `go/` | 官方 `github.com/modelcontextprotocol/go-sdk` v1.8.0 | `go/bin/hk-gov-mcp`（單一靜態 binary） |

兩者皆支援 **stdio**（預設）與 **Streamable HTTP**（stateless）兩種傳輸；initialize 實測協商至協議修訂 **2025-11-25**（現行官方 SDK 支援的最新修訂；官方規範站最新文檔為 2026-07-28）。全部工具標註 `readOnlyHint` annotation，並提供伺服器級 `instructions`。

## 工具清單（24 個）

所有工具均可選 `lang` 參數：`tc`（繁體中文，預設）/ `sc`（简体中文）/ `en`（English）。

### 氣象 — 香港天文台 HKO（data.weather.gov.hk）

| 工具 | 說明 |
|---|---|
| `get_current_weather` | 現時天氣：約 27 站氣溫、濕度、18 區雨量、UV 指數、天氣圖示、警告訊息 |
| `get_local_forecast` | 本地預報：大勢、熱帶氣旋資訊、今明預報、展望 |
| `get_9day_forecast` | 九天預報：每日天氣/氣溫/濕度/風力/顯著降雨概率 + 海水溫度 |
| `get_weather_warnings` | 生效中的天氣警告（摘要+詳情）及特別天氣提示；無警告時明確回覆 |

### 九巴 / 龍運 KMB（data.etabus.gov.hk）

| 工具 | 說明 |
|---|---|
| `get_kmb_route_list` · `route` | 按路線號查詢各變體（方向、起訖、service_type） |
| `get_kmb_route_stops` · `route, bound(O/I), service_type?` | 有序站名列表（含 stop id） |
| `get_kmb_route_eta` · `route, service_type?` | 全線實時到站（雙向，含備註） |
| `get_kmb_stop_eta` · `stop_id` | 單一車站所有路線實時到站 |

### 城巴 CTB（rt.data.gov.hk/v2/transport/citybus）

| 工具 | 說明 |
|---|---|
| `get_ctb_route_list` · `route` | 按路線號查詢（起訖） |
| `get_ctb_route_stops` · `route, direction(outbound/inbound)` | 有序站名列表 |
| `get_ctb_route_eta` · `route, direction` | 全線逐站實時到站（由 per-stop ETA 彙組） |
| `get_ctb_stop_eta` · `route, stop_id` | 單站單線實時到站 |

### 港鐵 MTR（rt.data.gov.hk/v1/transport/mtr）

| 工具 | 說明 |
|---|---|
| `get_mtr_schedule` · `line, station` | 重鐵實時班次（平台/方向、目的地、ttnt、延誤旗標） |
| `get_mtr_station_codes` · `query` | 以站名（中英）或代碼查 line/station code（內嵌 10 線 120 站對照表） |
| `get_mtr_frequency` · `line?` | 港鐵公佈嘅平均班次（平日朝/晚繁忙、非繁忙、星期六、星期日公眾假期），覆蓋全部重鐵線（含分段）及輕鐵路綫 — 由 mtr.com.hk 擷取嘅靜態快照（`scripts/scrape_mtr_frequencies.py`），可答「幾耐一班／嗰陣時仲有冇車」；並非實時 |
| `get_lrt_schedule` · `station_id` | 輕鐵實時班次（內嵌 68 站 id 對照表，如 100=兆康） |
| `get_mtr_bus_schedule` · `route` | 港鐵巴士實時班次（上游暫不穩定，會優雅回報） |

### 專線小巴 GMB（data.etagmb.gov.hk）

| 工具 | 說明 |
|---|---|
| `get_gmb_route_list` · `region?(HKI/KLN/NT), route?` | 路線號清單 |
| `get_gmb_route_variants` · `region, route_code` | 取 route_id 變體與方向（route_seq） |
| `get_gmb_route_stops` · `route_id, route_seq` | 有序站名列表（含 stop_id） |
| `get_gmb_eta` · `route_id, stop_id` | 實時到站 |

### 運輸署 TD

| 工具 | 說明 |
|---|---|
| `get_traffic_snapshot` · `camera_ids?` | 交通快拍圖像 URL（CID API） |
| `get_traffic_speed` | 主要路段實時速度/擠塞程度（速度地圖 XML） |
| `get_parking_vacancy` · `keyword?` | TD 參與停車場即時空位數（可按名稱/地區過濾） |

> **上游現況**：`get_mtr_bus_schedule`（404）、`get_traffic_snapshot`（403）、`get_traffic_speed`（503）所對應的政府端點在開發當下（2026-10）暫時故障或受限。工具會以可讀訊息（含官方端點）優雅回報，服務恢復後即自動正常；其餘 21 個工具已全部實測可用。

## 典型呼叫流程

- **巴士到站**：`get_kmb_route_list(1A)` → `get_kmb_route_stops(1A, O, 1)` → `get_kmb_route_eta(1A, 1)` 或 `get_kmb_stop_eta(stop_id)`
- **港鐵**：`get_mtr_station_codes("中環")` → `get_mtr_schedule(twl, cen)`
- **小巴**：`get_gmb_route_list(HKI, 1)` → `get_gmb_route_variants(HKI, "1")` → `get_gmb_route_stops(route_id, route_seq)` → `get_gmb_eta(route_id, stop_id)`
- **泊車**：`get_parking_vacancy("沙田")`

## 接入配置

### ZCode / Claude Desktop — TypeScript 版

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

### ZCode / Claude Desktop — Go 版

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

### Streamable HTTP 模式（遠端／多 client）

```bash
# TypeScript
node ts/dist/index.js --http --port 8819
# Go
go/bin/hk-gov-mcp -http 127.0.0.1:8819
```

接入配置改為 URL 型式：

```json
{ "mcpServers": { "hk-gov": { "url": "http://127.0.0.1:8819/mcp" } } }
```

## 開發與測試

```bash
# TypeScript
cd ts && npm install && npm run build && npm run smoke
npx @modelcontextprotocol/inspector node dist/index.js   # 人工複核

# Go
cd go && go build -o bin/hk-gov-mcp . && go test -v -timeout 12m .
```

冒煙測試會以 in-memory transport 直連伺服器，並逐一實呼 24 個工具（打真實上游 API）。

## 快取策略（兩版一致）

| 資料 | TTL |
|---|---|
| 實時到站 ETA | 20s |
| 現時天氣 / 警告 | 60s |
| 天氣預報 | 10 min |
| 速度地圖 | 2 min |
| 泊車空位 | 60s |
| 路線/站點靜態資料 | 6 h |

上游逾時 10s（城巴路線總表 30s）；錯誤統一回 `isError: true` 的可讀訊息，不會令 MCP 連線中斷。失敗結果快取 30s 以免衝擊故障中的上游。

## 資料來源

- 天文台：`data.weather.gov.hk/weatherAPI/opendata`（rhrread / flw / fnd / warnsum / warningInfo / swt）
- 九巴：`data.etabus.gov.hk`（v1 開放 API）
- 城巴 / 港鐵 / 輕鐵 / 港鐵巴士 / CID：`rt.data.gov.hk`
- 專線小巴：`data.etagmb.gov.hk`
- 運輸署：`resource.data.one.gov.hk/td/*`（速度地圖、停車場）、`api.data.gov.hk/v1/carpark-info-vacancy`
- 港鐵站碼表：由 `opendata.mtr.com.hk` CSV（`mtr_lines_and_stations.csv`、`light_rail_routes_and_stops.csv`）產生為內嵌 JSON（`ts/src/transport/data/`、`go/internal/hkapi/data/`）
