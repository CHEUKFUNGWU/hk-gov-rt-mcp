#!/usr/bin/env node
import http from "node:http";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { registerWeatherTools } from "./weather/tools.js";
import { registerTransportTools } from "./transport/tools.js";

const VERSION = "1.0.0";

export function createServer(): McpServer {
  const server = new McpServer(
    { name: "hk-gov-rt-mcp", version: VERSION },
    {
      instructions:
        "Real-time Hong Kong government open data. Weather tools (prefix get_) come from the Hong Kong Observatory; transport tools cover KMB, Citybus, MTR, Light Rail, MTR Bus, green minibuses and Transport Department traffic/parking feeds. " +
        "All tools accept lang: tc (繁體中文, default) / sc (简体中文) / en. Typical flows: bus ETA = route_list → route_stops/stop ids → route_eta or stop_eta; MTR = get_mtr_station_codes → get_mtr_schedule; GMB = route_list → route_variants → route_stops → eta.",
    },
  );
  registerWeatherTools(server);
  registerTransportTools(server);
  return server;
}

function argValue(flag: string): string | undefined {
  const i = process.argv.indexOf(flag);
  return i !== -1 ? process.argv[i + 1] : undefined;
}

async function main(): Promise<void> {
  if (process.argv.includes("--http")) {
    const port = Number(argValue("--port") ?? 8819);
    const server = createServer();
    const httpServer = http.createServer(async (req, res) => {
      const transport = new StreamableHTTPServerTransport({
        sessionIdGenerator: undefined,
        enableJsonResponse: true,
      });
      res.on("close", () => transport.close());
      await server.connect(transport);
      const chunks: Buffer[] = [];
      for await (const c of req) chunks.push(c as Buffer);
      const body = chunks.length > 0 ? JSON.parse(Buffer.concat(chunks).toString("utf8")) : undefined;
      await transport.handleRequest(req, res, body);
    });
    httpServer.listen(port, () => {
      console.error(`hk-gov-rt-mcp v${VERSION} listening on http://127.0.0.1:${port}/mcp (Streamable HTTP, stateless)`);
    });
    return;
  }
  const server = createServer();
  await server.connect(new StdioServerTransport());
}

main().catch((e) => {
  console.error("fatal:", e);
  process.exit(1);
});
