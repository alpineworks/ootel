package ootel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestNewOotelClient(t *testing.T) {
	client := NewOotelClient()
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewOotelClientWithTraceConfig(t *testing.T) {
	tc := NewTraceConfig(true, 1.0, "test-service", "1.0.0")
	client := NewOotelClient(WithTraceConfig(tc))

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.traceConfig == nil {
		t.Fatal("expected trace config to be set")
	}
	if client.traceConfig.ServiceName != "test-service" {
		t.Errorf("expected service name 'test-service', got '%s'", client.traceConfig.ServiceName)
	}
}

func TestNewOotelClientWithMetricConfig(t *testing.T) {
	mc := NewMetricConfig(true, ExporterTypeOTLPGRPC, 9090)
	client := NewOotelClient(WithMetricConfig(mc))

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.metricConfig == nil {
		t.Fatal("expected metric config to be set")
	}
	if client.metricConfig.ExporterType != ExporterTypeOTLPGRPC {
		t.Errorf("expected exporter type '%s', got '%s'", ExporterTypeOTLPGRPC, client.metricConfig.ExporterType)
	}
}

func TestInitWithDisabledConfigs(t *testing.T) {
	tc := NewTraceConfig(false, 1.0, "test-service", "1.0.0")
	mc := NewMetricConfig(false, ExporterTypeOTLPGRPC, 9090)
	client := NewOotelClient(WithTraceConfig(tc), WithMetricConfig(mc))

	ctx := context.Background()
	shutdown, err := client.Init(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestInitWithNoConfig(t *testing.T) {
	client := NewOotelClient()

	ctx := context.Background()
	shutdown, err := client.Init(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
}

func TestTraceProviderCreation(t *testing.T) {
	tc := NewTraceConfig(true, 1.0, "test-service", "1.0.0")
	client := NewOotelClient(WithTraceConfig(tc))

	ctx := context.Background()
	shutdown, err := client.Init(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	// Verify that a tracer can be obtained from the global provider
	tracer := otel.Tracer("test-tracer")
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}

	// Create a test span to verify the tracer works
	_, span := tracer.Start(ctx, "test-span")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()
}

func TestMeterProviderCreation(t *testing.T) {
	// Test OTLP GRPC meter provider without starting the HTTP server
	// by disabling the metric config (we test meter provider function directly)
	ctx := context.Background()

	// Test that meterProvider function works correctly for OTLP GRPC
	mp, err := meterProvider(ctx, ExporterTypeOTLPGRPC)
	if err != nil {
		t.Fatalf("failed to create OTLP GRPC meter provider: %v", err)
	}
	if mp == nil {
		t.Fatal("expected non-nil meter provider")
	}
	_ = mp.Shutdown(ctx)

	// Test that meterProvider function works correctly for OTLP HTTP
	mp, err = meterProvider(ctx, ExporterTypeOTLPHTTP)
	if err != nil {
		t.Fatalf("failed to create OTLP HTTP meter provider: %v", err)
	}
	if mp == nil {
		t.Fatal("expected non-nil meter provider")
	}
	_ = mp.Shutdown(ctx)

	// Test that meterProvider function works correctly for Prometheus
	mp, err = meterProvider(ctx, ExporterTypePrometheus)
	if err != nil {
		t.Fatalf("failed to create Prometheus meter provider: %v", err)
	}
	if mp == nil {
		t.Fatal("expected non-nil meter provider")
	}
	_ = mp.Shutdown(ctx)
}

func TestMeterFunctionality(t *testing.T) {
	ctx := context.Background()

	// Create meter provider directly to avoid HTTP server conflicts
	mp, err := meterProvider(ctx, ExporterTypeOTLPGRPC)
	if err != nil {
		t.Fatalf("failed to create meter provider: %v", err)
	}
	defer func() { _ = mp.Shutdown(ctx) }()

	// Set as global provider temporarily
	otel.SetMeterProvider(mp)

	// Verify that a meter can be obtained
	meter := otel.Meter("test-meter")
	if meter == nil {
		t.Fatal("expected non-nil meter")
	}

	// Create a test counter to verify the meter works
	counter, err := meter.Int64Counter("test_counter")
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}
	counter.Add(ctx, 1)

	// Create a histogram to verify more complex instruments work
	histogram, err := meter.Float64Histogram("test_histogram")
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}
	histogram.Record(ctx, 1.5)
}

func TestInvalidExporterType(t *testing.T) {
	ctx := context.Background()
	_, err := meterProvider(ctx, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid exporter type")
	}
}

func TestTraceConfigValues(t *testing.T) {
	tc := NewTraceConfig(true, 0.5, "my-service", "2.0.0")

	if !tc.Enabled {
		t.Error("expected Enabled to be true")
	}
	if tc.SampleRate != 0.5 {
		t.Errorf("expected SampleRate 0.5, got %f", tc.SampleRate)
	}
	if tc.ServiceName != "my-service" {
		t.Errorf("expected ServiceName 'my-service', got '%s'", tc.ServiceName)
	}
	if tc.ServiceVersion != "2.0.0" {
		t.Errorf("expected ServiceVersion '2.0.0', got '%s'", tc.ServiceVersion)
	}
}

func TestMetricConfigValues(t *testing.T) {
	mc := NewMetricConfig(true, ExporterTypePrometheus, 8080)

	if !mc.Enabled {
		t.Error("expected Enabled to be true")
	}
	if mc.ExporterType != ExporterTypePrometheus {
		t.Errorf("expected ExporterType '%s', got '%s'", ExporterTypePrometheus, mc.ExporterType)
	}
	if mc.ServerPort != 8080 {
		t.Errorf("expected ServerPort 8080, got %d", mc.ServerPort)
	}
}
