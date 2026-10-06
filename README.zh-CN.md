[English](README.md) | [繁體中文（香港）](README.zh-HK.md) | [简体中文](README.zh-CN.md)

# hk-gov-rt-mcp

将香港政府 data.gov.hk（气象 + 运输分类）的实时开放数据 API 封装为 MCP（Model Context Protocol）服务器。

**双实现**：同一份工具契约，两种语言实现，工具、参数与输出格式完全一致：

| 实现 | 位置 | 技术 | 启动 |
|---|---|---|---|
| TypeScript | `ts/` | 官方 `@modelcontextprotocol/sdk` ^1.32 + zod + Node ≥20 | `node ts/dist/index.js` |
| Go | `go/` | 官方 `github.com/modelcontextprotocol/go-sdk` v1.8.0 | `go/bin/hk-gov-mcp`（单一静态二进制） |

两者均支持 **stdio**（默认）与 **Streamable HTTP**（无状态）两种传输；initialize 实测协商至协议修订版 **2025-11-25**（当前官方 SDK 支持的最新修订；官方规范站最新文档为 2026-07-28）。所有工具均标注 `readOnlyHint` 注解，并提供服务器级 `instructions`。

## 工具列表（23 个）

所有工具均可选 `lang` 参数：`tc`（繁體中文，默认）/ `sc`（简体中文）/ `en`（English）。

### 气象 — 香港天文台 HKO（data.weather.gov.hk）

| 工具 | 说明 |
|---|---|
| `get_current_weather` | 现时天气：约 27 个站点的气温、湿度、18 区雨量、紫外线指数、天气图标、警告信息 |
| `get_local_forecast` | 本地预报：大势、热带气旋资讯、今明预报、展望 |
| `get_9day_forecast` | 九天预报：每日天气/气温/湿度/风力/显著降雨概率 + 海水温度 |
| `get_weather_warnings` | 生效中的天气警告（摘要+详情）及特别天气提示；无警告时明确回复 |

### 九巴 / 龙运 KMB（data.etabus.gov.hk）

| 工具 | 说明 |
|---|---|
| `get_kmb_route_list` · `route` | 按路线号查询各变体（方向、起讫、service_type） |
| `get_kmb_route_stops` · `route, bound(O/I), service_type?` | 有序站名列表（含 stop id） |
| `get_kmb_route_eta` · `route, service_type?` | 全线实时到站（双向，含备注） |
| `get_kmb_stop_eta` · `stop_id` | 单一车站所有路线实时到站 |

### 城巴 CTB（rt.data.gov.hk/v2/transport/citybus）

| 工具 | 说明 |
|---|---|
| `get_ctb_route_list` · `route` | 按路线号查询（起讫） |
| `get_ctb_route_stops` · `route, direction(outbound/inbound)` | 有序站名列表 |
| `get_ctb_route_eta` · `route, direction` | 全线逐站实时到站（由 per-stop ETA 汇组） |
| `get_ctb_stop_eta` · `route, stop_id` | 单站单线实时到站 |

### 港铁 MTR（rt.data.gov.hk/v1/transport/mtr）

| 工具 | 说明 |
|---|---|
| `get_mtr_schedule` · `line, station` | 重铁实时班次（月台/方向、目的地、ttnt、延误标志） |
| `get_mtr_station_codes` · `query` | 以站名（中英）或代码查询 line/station code（内置 10 线 120 站对照表） |
| `get_lrt_schedule` · `station_id` | 轻铁实时班次（内置 68 站 id 对照表，如 100=兆康） |
| `get_mtr_bus_schedule` · `route` | 港铁巴士实时班次（上游暂不稳定，会优雅回报） |

### 专线小巴 GMB（data.etagmb.gov.hk）

| 工具 | 说明 |
|---|---|
| `get_gmb_route_list` · `region?(HKI/KLN/NT), route?` | 路线号清单 |
| `get_gmb_route_variants` · `region, route_code` | 获取 route_id 变体与方向（route_seq） |
| `get_gmb_route_stops` · `route_id, route_seq` | 有序站名列表（含 stop_id） |
| `get_gmb_eta` · `route_id, stop_id` | 实时到站 |

### 运输署 TD

| 工具 | 说明 |
|---|---|
| `get_traffic_snapshot` · `camera_ids?` | 交通快拍图像 URL（CID API） |
| `get_traffic_speed` | 主要路段实时速度/拥堵程度（速度地图 XML） |
| `get_parking_vacancy` · `keyword?` | TD 参与停车场的实时空位数（可按名称/地区过滤） |

> **上游现状**：`get_mtr_bus_schedule`（404）、`get_traffic_snapshot`（403）、`get_traffic_speed`（503）对应的政府端点在开发时（2026-10）暂时故障或受限。工具会以可读信息（含官方端点）优雅回报，服务恢复后即自动正常；其余 20 个工具已全部实测可用。

## 典型调用流程

- **巴士到站**：`get_kmb_route_list(1A)` → `get_kmb_route_stops(1A, O, 1)` → `get_kmb_route_eta(1A, 1)` 或 `get_kmb_stop_eta(stop_id)`
- **港铁**：`get_mtr_station_codes("中环")` → `get_mtr_schedule(twl, cen)`
- **小巴**：`get_gmb_route_list(HKI, 1)` → `get_gmb_route_variants(HKI, "1")` → `get_gmb_route_stops(route_id, route_seq)` → `get_gmb_eta(route_id, stop_id)`
- **泊车**：`get_parking_vacancy("沙田")`

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

### Streamable HTTP 模式（远程／多客户端）

```bash
# TypeScript
node ts/dist/index.js --http --port 8819
# Go
go/bin/hk-gov-mcp -http 127.0.0.1:8819
```

接入配置改为 URL 形式：

```json
{ "mcpServers": { "hk-gov": { "url": "http://127.0.0.1:8819/mcp" } } }
```

## 开发与测试

```bash
# TypeScript
cd ts && npm install && npm run build && npm run smoke
npx @modelcontextprotocol/inspector node dist/index.js   # 人工复核

# Go
cd go && go build -o bin/hk-gov-mcp . && go test -v -timeout 12m .
```

冒烟测试会以 in-memory transport 直连服务器，并逐一实呼 23 个工具（打真实上游 API）。

## 缓存策略（两版一致）

| 数据 | TTL |
|---|---|
| 实时到站 ETA | 20s |
| 现时天气 / 警告 | 60s |
| 天气预报 | 10 min |
| 速度地图 | 2 min |
| 泊车空位 | 60s |
| 路线/站点静态数据 | 6 h |

上游超时 10s（城巴路线总表 30s）；错误统一返回 `isError: true` 的可读信息，不会令 MCP 连接中断。失败结果缓存 30s，以免冲击故障中的上游。

## 数据来源

- 天文台：`data.weather.gov.hk/weatherAPI/opendata`（rhrread / flw / fnd / warnsum / warningInfo / swt）
- 九巴：`data.etabus.gov.hk`（v1 开放 API）
- 城巴 / 港铁 / 轻铁 / 港铁巴士 / CID：`rt.data.gov.hk`
- 专线小巴：`data.etagmb.gov.hk`
- 运输署：`resource.data.one.gov.hk/td/*`（速度地图、停车场）、`api.data.gov.hk/v1/carpark-info-vacancy`
- 港铁站码表：由 `opendata.mtr.com.hk` CSV（`mtr_lines_and_stations.csv`、`light_rail_routes_and_stops.csv`）生成为内嵌 JSON（`ts/src/transport/data/`、`go/internal/hkapi/data/`）
