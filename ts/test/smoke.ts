/** Smoke test: connect an MCP client to the server over in-memory transport
 *  and call every tool against the live upstream APIs.
 *  Run: npm run smoke  (or npx tsx test/smoke.ts) */
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";

import { createServer } from "../src/index.js";

const CLIENT = new Client({ name: "smoke", version: "0.0.1" });

interface Call {
  tool: string;
  args: Record<string, unknown>;
}

const calls: Call[] = [
  // weather
  { tool: "get_current_weather", args: { lang: "tc" } },
  { tool: "get_local_forecast", args: { lang: "tc" } },
  { tool: "get_9day_forecast", args: { lang: "en" } },
  { tool: "get_weather_warnings", args: { lang: "tc" } },
  // KMB
  { tool: "get_kmb_route_list", args: { route: "1A", lang: "tc" } },
  { tool: "get_kmb_route_stops", args: { route: "1A", bound: "O", service_type: "1", lang: "tc" } },
  { tool: "get_kmb_route_eta", args: { route: "1A", service_type: "1", lang: "tc" } },
  { tool: "get_kmb_stop_eta", args: { stop_id: "184832", lang: "tc" } },
  // CTB
  { tool: "get_ctb_route_list", args: { route: "1", lang: "tc" } },
  { tool: "get_ctb_route_stops", args: { route: "1", direction: "outbound", lang: "tc" } },
  { tool: "get_ctb_route_eta", args: { route: "1", direction: "outbound", lang: "tc" } },
  { tool: "get_ctb_stop_eta", args: { route: "1", stop_id: "001027", lang: "tc" } },
  // MTR
  { tool: "get_mtr_station_codes", args: { query: "中環", lang: "tc" } },
  { tool: "get_mtr_frequency", args: { line: "TWL", lang: "tc" } },
  { tool: "get_mtr_frequency", args: { lang: "en" } },
  { tool: "get_mtr_schedule", args: { line: "twl", station: "cen", lang: "tc" } },
  { tool: "get_lrt_schedule", args: { station_id: "100", lang: "tc" } },
  { tool: "get_mtr_bus_schedule", args: { route: "K51", lang: "tc" } },
  // GMB
  { tool: "get_gmb_route_list", args: { region: "HKI", route: "1", lang: "tc" } },
  { tool: "get_gmb_route_variants", args: { region: "HKI", route_code: "1", lang: "tc" } },
  { tool: "get_gmb_route_stops", args: { route_id: "2006408", route_seq: "1", lang: "tc" } },
  { tool: "get_gmb_eta", args: { route_id: "2006408", stop_id: "20014489", lang: "tc" } },
  // TD
  { tool: "get_traffic_snapshot", args: { camera_ids: ["A01"], lang: "tc" } },
  { tool: "get_traffic_speed", args: { lang: "tc" } },
  { tool: "get_parking_vacancy", args: { keyword: "沙田", lang: "tc" } },
];

async function main(): Promise<void> {
  const [clientT, serverT] = InMemoryTransport.createLinkedPair();
  const server = createServer();
  await server.connect(serverT);
  await CLIENT.connect(clientT);

  const { tools } = await CLIENT.listTools();
  console.log(`server exposes ${tools.length} tools:`);
  console.log(tools.map((t) => t.name).join(", "));

  let failed = 0;
  for (const c of calls) {
    const t0 = Date.now();
    try {
      const res = await CLIENT.callTool({ name: c.tool, arguments: c.args });
      const isError = res.isError === true;
      const text = ((res.content as { type: string; text?: string }[] | undefined) ?? [])
        .map((b) => b.text ?? "")
        .join("\n");
      const ms = Date.now() - t0;
      if (isError) failed++;
      console.log(
        `${isError ? "FAIL" : "PASS"} ${c.tool} (${ms}ms)\n  ${text.split("\n").slice(0, 4).join("\n  ").slice(0, 400)}`,
      );
    } catch (e) {
      failed++;
      console.log(`ERROR ${c.tool}: ${String(e)}`);
    }
  }
  console.log(failed === 0 ? "\nALL TOOLS OK" : `\n${failed} tool call(s) returned errors`);
  process.exit(failed === 0 ? 0 : 1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
