package telemetry

import (
	"context"
	"fmt"
	"path"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TracingUnaryInterceptor returns a grpc.UnaryServerInterceptor that adds OpenTelemetry tracing
func TracingUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	tracer := otel.GetTracerProvider().Tracer("grpc-server")

	name := path.Base(info.FullMethod)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("grpc.unary.%s", name),
		trace.WithAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("rpc.service", path.Dir(info.FullMethod)),
		),
	)
	defer span.End()

	start := time.Now()
	resp, err := handler(ctx, req)
	elapsed := time.Since(start)

	// Add method name, status code and execution time as attributes
	span.SetAttributes(
		attribute.String("method", name),
		attribute.Int64("duration_ms", elapsed.Milliseconds()),
	)

	if err != nil {
		st, _ := status.FromError(err)
		span.SetStatus(codes.Error, st.Message())
		span.RecordError(err)
	}

	return resp, err
}

// TracingStreamInterceptor returns a grpc.StreamServerInterceptor that adds OpenTelemetry tracing
func TracingStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	tracer := otel.GetTracerProvider().Tracer("grpc-server")

	name := path.Base(info.FullMethod)
	ctx := ss.Context()

	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("grpc.stream.%s", name),
		trace.WithAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("rpc.service", path.Dir(info.FullMethod)),
		),
	)
	defer span.End()

	// Wrap the server stream to propagate the context with the span
	wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

	start := time.Now()
	err := handler(srv, wrapped)
	elapsed := time.Since(start)

	// Add method name, status code and execution time as attributes
	span.SetAttributes(
		attribute.String("method", name),
		attribute.Int64("duration_ms", elapsed.Milliseconds()),
	)

	if err != nil {
		st, _ := status.FromError(err)
		span.SetStatus(codes.Error, st.Message())
		span.RecordError(err)
	}

	return err
}

// wrappedServerStream wraps grpc.ServerStream to propagate the modified context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// ExtractTraceInfoFromMetadata extracts trace context from gRPC metadata
func ExtractTraceInfoFromMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, metadata.MD(md))
}