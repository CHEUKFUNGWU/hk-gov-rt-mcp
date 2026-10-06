// hk-gov-mcp: MCP server wrapping Hong Kong government real-time weather and
// transport open data (Go implementation; tool contract mirrors ../ts).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"hk-gov-rt-mcp/go/internal/tools"
)

const version = "1.0.0"

func newServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "hk-gov-rt-mcp", Version: version}, &mcp.ServerOptions{
		Instructions: "Real-time Hong Kong government open data. Weather tools (prefix get_) come from the Hong Kong Observatory; transport tools cover KMB, Citybus, MTR, Light Rail, MTR Bus, green minibuses and Transport Department traffic/parking feeds. " +
			"All tools accept lang: tc (繁體中文, default) / sc (简体中文) / en. Typical flows: bus ETA = route_list → route_stops/stop ids → route_eta or stop_eta; MTR = get_mtr_station_codes → get_mtr_schedule; GMB = route_list → route_variants → route_stops → eta.",
	})
	tools.Register(server)
	return server
}

func main() {
	httpAddr := flag.String("http", "", "serve Streamable HTTP (stateless) on the given address instead of stdio, e.g. 127.0.0.1:8819")
	flag.Parse()

	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newServer() }, nil)
		fmt.Fprintf(log.Writer(), "hk-gov-rt-mcp v%s listening on http://%s/mcp (Streamable HTTP, stateless)\n", version, *httpAddr)
		log.Fatal(http.ListenAndServe(*httpAddr, handler))
		return
	}

	ctx := context.Background()
	if err := newServer().Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
