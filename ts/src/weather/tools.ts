import { langParam, type Lang } from "../lang.js";
import { defineTool, registerTools } from "../mcp.js";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import * as hko from "./hko.js";

const langShape = { lang: langParam } as const;

export function registerWeatherTools(server: McpServer): void {
  registerTools(server, [
    defineTool(
      "get_current_weather",
      "HKO current weather report",
      "Hong Kong Observatory current weather report: temperatures at ~27 stations, humidity, rainfall by district, UV index, weather icon and any warning messages. Updates every ~10 minutes.",
      langShape,
      (a: { lang: Lang }) => hko.formatCurrent(a.lang),
    ),
    defineTool(
      "get_local_forecast",
      "HKO local weather forecast",
      "Hong Kong Observatory local forecast: general situation, tropical cyclone info, today/tonight/tomorrow forecast description and outlook. Updates roughly hourly.",
      langShape,
      (a: { lang: Lang }) => hko.formatLocal(a.lang),
    ),
    defineTool(
      "get_9day_forecast",
      "HKO 9-day forecast",
      "Hong Kong Observatory 9-day weather forecast: daily weather, min/max temperature, humidity range, wind, probability of significant rain (PSR), plus sea and soil temperatures.",
      langShape,
      (a: { lang: Lang }) => hko.formatNineDay(a.lang),
    ),
    defineTool(
      "get_weather_warnings",
      "HKO weather warnings",
      "Weather warnings currently in force (summary + details) and HKO special weather tips. Returns an explicit 'no warnings' message when none are active.",
      langShape,
      (a: { lang: Lang }) => hko.formatWarnings(a.lang),
    ),
  ]);
}
