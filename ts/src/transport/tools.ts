import { z } from "zod";
import { langParam, type Lang } from "../lang.js";
import { defineTool, registerTools } from "../mcp.js";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import * as kmb from "./kmb.js";
import * as ctb from "./ctb.js";
import * as mtr from "./mtr.js";
import * as gmb from "./gmb.js";
import * as td from "./traffic.js";
import * as parking from "./parking.js";

const langShape = { lang: langParam } as const;

const routeParam = z.string().min(1).describe("Route number, e.g. 1A, 269D, A10");
const boundParam = z.enum(["O", "I"]).describe("O=outbound (去程), I=inbound (回程)");
const serviceTypeParam = z.string().default("1").describe('KMB service type variant, default "1" (normally leave as-is)');

export function registerTransportTools(server: McpServer): void {
  registerTools(server, [
    /* ---- KMB / Long Win (data.etabus.gov.hk) ---- */
    defineTool(
      "get_kmb_route_list",
      "KMB route lookup",
      "Look up KMB / Long Win bus routes by route number (prefix match) and get each variant's direction, origin, destination and service_type. route is required to keep the response small.",
      { route: routeParam, lang: langShape.lang },
      (a: { route: string; lang: Lang }) =>
        kmb.findRouteVariants(a.route, a.lang).then((vs) =>
          vs.length === 0
            ? `No KMB route matching "${a.route}".`
            : vs
                .map(
                  (v) =>
                    `${v.route} [${kmb.BOUND_LABEL[a.lang][v.bound] ?? v.bound}] service_type ${v.service_type}: ${v.orig} → ${v.dest} (${v.co})`,
                )
                .join("\n"),
        ),
    ),
    defineTool(
      "get_kmb_route_stops",
      "KMB route stops",
      "Ordered stop list of a KMB route variant (from get_kmb_route_list): seq, stop name and stop id.",
      { route: routeParam, bound: boundParam, service_type: serviceTypeParam, lang: langShape.lang },
      (a: { route: string; bound: "O" | "I"; service_type: string; lang: Lang }) =>
        kmb.routeStops(a.route, a.bound, a.service_type, a.lang),
    ),
    defineTool(
      "get_kmb_route_eta",
      "KMB route ETAs",
      "Real-time arrival times for every stop of a KMB route (both directions), with destination and remark. Updates every minute.",
      { route: routeParam, service_type: serviceTypeParam, lang: langShape.lang },
      (a: { route: string; service_type: string; lang: Lang }) =>
        kmb.routeEta(a.route, a.service_type, a.lang),
    ),
    defineTool(
      "get_kmb_stop_eta",
      "KMB stop ETA",
      "Real-time ETAs of all routes serving a KMB stop. Get stop ids from get_kmb_route_stops.",
      { stop_id: z.string().min(1).describe("KMB stop id, e.g. 184832"), lang: langShape.lang },
      (a: { stop_id: string; lang: Lang }) => kmb.stopEta(a.stop_id, a.lang),
    ),

    /* ---- Citybus (rt.data.gov.hk/v2/transport/citybus) ---- */
    defineTool(
      "get_ctb_route_list",
      "Citybus route lookup",
      "Look up Citybus (CTB) routes by route number (prefix match) with origin/destination. route is required to keep the response small.",
      { route: routeParam, lang: langShape.lang },
      (a: { route: string; lang: Lang }) =>
        ctb.findRoutes(a.route, a.lang).then((rs) =>
          rs.length === 0
            ? `No CTB route matching "${a.route}".`
            : rs
                .map((r) => `${r.route}: ${r["orig_" + a.lang] ?? r.orig_en} → ${r["dest_" + a.lang] ?? r.dest_en}`)
                .join("\n"),
        ),
    ),
    defineTool(
      "get_ctb_route_stops",
      "Citybus route stops",
      "Ordered stop list of a Citybus route direction (inbound/outbound): seq, stop name and stop id.",
      { route: routeParam, direction: z.enum(["outbound", "inbound"]).describe("outbound=去程, inbound=回程"), lang: langShape.lang },
      (a: { route: string; direction: "outbound" | "inbound"; lang: Lang }) =>
        ctb.routeStops(a.route, a.direction, a.lang),
    ),
    defineTool(
      "get_ctb_route_eta",
      "Citybus route ETAs",
      "Real-time ETAs for every stop of a Citybus route direction (assembled stop-by-stop from the CTB per-stop ETA API).",
      { route: routeParam, direction: z.enum(["outbound", "inbound"]), lang: langShape.lang },
      (a: { route: string; direction: "outbound" | "inbound"; lang: Lang }) =>
        ctb.routeEta(a.route, a.direction, a.lang),
    ),
    defineTool(
      "get_ctb_stop_eta",
      "Citybus stop ETA",
      "Real-time ETAs of a Citybus stop for one route. Get stop ids from get_ctb_route_stops.",
      { route: routeParam, stop_id: z.string().min(1).describe('Citybus stop id, e.g. "001027"'), lang: langShape.lang },
      (a: { route: string; stop_id: string; lang: Lang }) => ctb.stopEta(a.route, a.stop_id, a.lang),
    ),

    /* ---- MTR (rt.data.gov.hk/v1/transport/mtr) ---- */
    defineTool(
      "get_mtr_schedule",
      "MTR train schedule",
      "Real-time MTR heavy rail train schedule for one line+station: next trains per platform/direction with destination, minutes-to-train (ttnt) and service-delay flag. Use get_mtr_station_codes to find line/station codes.",
      {
        line: z.string().min(1).describe("MTR line code, e.g. twl (Tsuen Wan), eal, ktl, isl, tcl, tml, sil, ael, tkl, drl"),
        station: z.string().min(1).describe("Station code on that line, e.g. cen (Central). Comma-separated codes allowed."),
        lang: langShape.lang,
      },
      (a: { line: string; station: string; lang: Lang }) => mtr.trainSchedule(a.line, a.station, a.lang),
    ),
    defineTool(
      "get_mtr_station_codes",
      "MTR station code lookup",
      "Find MTR line and station codes by (partial) station name or exact code — needed before calling get_mtr_schedule.",
      { query: z.string().min(1).describe("Station name fragment (any language) or station code"), lang: langShape.lang },
      (a: { query: string; lang: Lang }) => mtr.lookupStationCodes(a.query, a.lang),
    ),
    defineTool(
      "get_lrt_schedule",
      "Light Rail schedule",
      "Real-time MTR Light Rail schedule for one stop: routes, destinations and arrival times per platform. Get stop ids from the embedded table (error messages list sample ids).",
      { station_id: z.string().min(1).describe("LRT numeric stop id, e.g. 100"), lang: langShape.lang },
      (a: { station_id: string; lang: Lang }) => mtr.lrtSchedule(a.station_id, a.lang),
    ),
    defineTool(
      "get_mtr_bus_schedule",
      "MTR bus schedule",
      "MTR Bus / feeder bus real-time schedule by route id (e.g. K51). Upstream has been unstable; unavailability is reported gracefully.",
      { route: z.string().min(1).describe("MTR bus route id, e.g. K51"), lang: langShape.lang },
      (a: { route: string; lang: Lang }) => mtr.busSchedule(a.route),
    ),

    /* ---- Green Minibus (data.etagmb.gov.hk) ---- */
    defineTool(
      "get_gmb_route_list",
      "GMB route list",
      "List green minibus (專線小巴) route numbers, optionally filtered by region (HKI/KLN/NT) and route-number prefix. Use get_gmb_route_variants next to obtain route_id.",
      {
        region: z.enum(["HKI", "KLN", "NT"]).optional().describe("Region: HKI=香港島, KLN=九龍, NT=新界"),
        route: z.string().optional().describe("Route number prefix filter"),
      },
      (a: { region?: "HKI" | "KLN" | "NT"; route?: string }) => gmb.routeList(a.region, a.route),
    ),
    defineTool(
      "get_gmb_route_variants",
      "GMB route variants",
      "Get route_id variants (with directions and route_seq) for a green minibus route number — needed before get_gmb_route_stops.",
      { region: z.enum(["HKI", "KLN", "NT"]), route_code: z.string().min(1).describe("GMB route number, e.g. 1"), lang: langShape.lang },
      (a: { region: "HKI" | "KLN" | "NT"; route_code: string; lang: Lang }) =>
        gmb.routeVariants(a.region, a.route_code, a.lang),
    ),
    defineTool(
      "get_gmb_route_stops",
      "GMB route stops",
      "Ordered stop list of a green minibus route (route_id + route_seq from get_gmb_route_variants).",
      { route_id: z.string().min(1).describe("GMB numeric route_id, e.g. 2006408"), route_seq: z.enum(["1", "2"]).describe("Direction from variants"), lang: langShape.lang },
      (a: { route_id: string; route_seq: "1" | "2"; lang: Lang }) => gmb.routeStops(a.route_id, a.route_seq, a.lang),
    ),
    defineTool(
      "get_gmb_eta",
      "GMB stop ETA",
      "Real-time ETA of a green minibus stop (route_id + stop_id from get_gmb_route_stops).",
      { route_id: z.string().min(1), stop_id: z.string().min(1).describe("GMB numeric stop_id"), lang: langShape.lang },
      (a: { route_id: string; stop_id: string; lang: Lang }) => gmb.eta(a.route_id, a.stop_id, a.lang),
    ),

    /* ---- Transport Department ---- */
    defineTool(
      "get_traffic_snapshot",
      "TD traffic camera snapshots",
      "Live traffic camera snapshot image URLs from Transport Department (CID API). Provide camera ids like A01, KE1, NT2.",
      {
        camera_ids: z.array(z.string()).max(20).optional().describe('Camera ids, e.g. ["A01","KE1"]'),
        lang: langShape.lang,
      },
      (a: { camera_ids?: string[]; lang: Lang }) => td.cameraSnapshots(a.camera_ids ?? ["A01"], a.lang),
    ),
    defineTool(
      "get_traffic_speed",
      "TD traffic speed map",
      "Real-time average speed and saturation level (GOOD/AVERAGE/BAD) for major road links from the TD speed map feed. Upstream has been returning 503 lately; reported gracefully.",
      { lang: langShape.lang },
      () => td.trafficSpeed(),
    ),
    defineTool(
      "get_parking_vacancy",
      "TD car park vacancy",
      "Live vacant-space counts for TD participating car parks (hourly category), searchable by car park name / district keyword.",
      { keyword: z.string().optional().describe("Filter by car park name or district fragment"), lang: langShape.lang },
      (a: { keyword?: string; lang: Lang }) => parking.parkingVacancy(a.keyword, a.lang),
    ),
  ]);
}
