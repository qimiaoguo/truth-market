package middleware

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc/stats"
)

// GRPCServerHandler returns a stats.Handler that instruments inbound gRPC
// RPCs with OpenTelemetry tracing and metrics. Register it on the server via
// grpc.NewServer(grpc.StatsHandler(middleware.GRPCServerHandler())).
func GRPCServerHandler(opts ...otelgrpc.Option) stats.Handler {
	return otelgrpc.NewServerHandler(opts...)
}

// GRPCClientHandler returns a stats.Handler that instruments outbound gRPC
// RPCs with OpenTelemetry tracing and metrics. Register it on the client via
// grpc.NewClient(addr, grpc.WithStatsHandler(middleware.GRPCClientHandler())).
func GRPCClientHandler(opts ...otelgrpc.Option) stats.Handler {
	return otelgrpc.NewClientHandler(opts...)
}
