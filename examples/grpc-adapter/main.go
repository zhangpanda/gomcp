// Command grpc-adapter shows how to import a gRPC service as MCP tools
// using adapter.ImportGRPC.
//
// The example wires the standard grpc.health.v1.Health service into an
// MCP stdio server so Claude Desktop / Cursor / Kiro can call
// HealthCheck as a tool. No protoc is required — we reuse the
// generated types shipped with google.golang.org/grpc/health.
//
// Flow:
//  1. Start a gRPC server on a loopback port.
//  2. Register the Health service + reflection (ImportGRPC discovers
//     services via server reflection).
//  3. Dial the server with grpc.NewClient.
//  4. adapter.ImportGRPC registers each unary method as an MCP tool.
//     Streaming methods are skipped.
//  5. Serve MCP over stdio.
//
// The resulting tool is "health.check" (service name snake-cased +
// method name snake-cased). Call it with {"service":""} for the
// overall SERVING status, or {"service":"example.svc"} for a named
// component.
//
// Usage with Claude Desktop:
//
//	{
//	  "mcpServers": {
//	    "grpc-health": {
//	      "command": "go",
//	      "args": ["run", "./examples/grpc-adapter"]
//	    }
//	  }
//	}
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// buildServer wires the gRPC backend and the MCP server together,
// returning the MCP server plus a cleanup function that tears down
// both the gRPC server and the dialed client. Separated from main()
// so tests can drive the same setup without calling Stdio().
func buildServer() (*gomcp.Server, func(), error) {
	// 1. start gRPC on loopback:0 so parallel runs don't collide.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("listen: %w", err)
	}
	grpcAddr := lis.Addr().String()

	gsrv := grpc.NewServer()
	hsrv := health.NewServer()
	// Mark the overall status + a named component as SERVING so
	// callers see something interesting rather than the default
	// SERVICE_UNKNOWN.
	hsrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	hsrv.SetServingStatus("example.svc", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(gsrv, hsrv)
	// Reflection is what ImportGRPC uses to discover services.
	reflection.Register(gsrv)

	serveErr := make(chan error, 1)
	go func() { serveErr <- gsrv.Serve(lis) }()

	// 2. dial ourselves. Insecure credentials are fine here — it is
	// loopback only and the connection is not reused outside this
	// process.
	conn, err := grpc.NewClient(grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		gsrv.Stop()
		return nil, nil, fmt.Errorf("dial: %w", err)
	}

	// 3. build the MCP server and import all unary gRPC methods.
	s := gomcp.New("grpc-adapter-demo", "1.0.0")
	s.Use(gomcp.Logger())

	if err := adapter.ImportGRPC(s, conn, adapter.GRPCOptions{
		// Bound reflection discovery: a wedged server must not hang
		// startup forever.
		Timeout: 10 * time.Second,
		// Filter explicitly to the health service. ImportGRPC already
		// drops reflection itself, but being explicit documents intent
		// and keeps this example focused.
		Services: []string{"grpc.health.v1.Health"},
	}); err != nil {
		_ = conn.Close()
		gsrv.Stop()
		return nil, nil, fmt.Errorf("import grpc: %w", err)
	}

	log.Printf("grpc-adapter: imported gRPC at %s as MCP tools", grpcAddr)

	cleanup := func() {
		_ = conn.Close()
		gsrv.GracefulStop()
	}
	return s, cleanup, nil
}

func main() {
	s, cleanup, err := buildServer()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	if err := s.Stdio(); err != nil {
		log.Fatal(err)
	}
}
