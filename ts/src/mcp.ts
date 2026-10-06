import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { ZodRawShape } from "zod";
import { z } from "zod";

/** All tools in this server are read-only fetches of open data. */
const READ_ONLY_ANNOTATIONS = {
  title: undefined,
  readOnlyHint: true,
  destructiveHint: false,
  idempotentHint: true,
  openWorldHint: true,
} as const;

export interface ToolResult {
  /** LLM-facing text content. */
  text: string;
}

export interface ToolDef {
  name: string;
  title: string;
  description: string;
  shape: ZodRawShape;
  run: (args: Record<string, unknown>) => Promise<ToolResult>;
}

/** Type-safe tool definition: the run callback receives the Zod-inferred args
 *  while the stored definition stays erased for uniform registration. */
export function defineTool<S extends ZodRawShape>(
  name: string,
  title: string,
  description: string,
  shape: S,
  run: (args: z.infer<z.ZodObject<S>>) => Promise<string>,
): ToolDef {
  return {
    name,
    title,
    description,
    shape,
    run: (raw) => run(raw as z.infer<z.ZodObject<S>>).then((text) => ({ text })),
  };
}

/** Wrap a tool definition with uniform upstream-error handling so a failing
 *  API never breaks the MCP connection. */
export function registerTools(server: McpServer, tools: ToolDef[]): void {
  for (const t of tools) {
    server.registerTool(
      t.name,
      {
        title: t.title,
        description: t.description,
        inputSchema: t.shape,
        annotations: { ...READ_ONLY_ANNOTATIONS, title: t.title },
      },
      async (args) => {
        try {
          const r = await t.run(args);
          return { content: [{ type: "text", text: r.text }] };
        } catch (e) {
          const msg = e instanceof Error ? e.message : String(e);
          return {
            content: [{ type: "text", text: `Error: ${msg}` }],
            isError: true,
          };
        }
      },
    );
  }
}

/** Compact JSON text helper — drops undefined, keeps stable key order. */
export function jsonText(value: unknown): ToolResult {
  return { text: JSON.stringify(value, null, 1) };
}

export function textResult(text: string): ToolResult {
  return { text };
}
