package ootel

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"alpineworks.io/ootel/healthcheck"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	ExporterTypePrometheus = "prometheus"
	ExporterTypeOTLPGRPC   = "otlpgrpc"
	ExporterTypeOTLPHTTP   = "otlphttp"
)

type traceConfig struct {
	Enabled        bool
	SampleRate     float64
	ServiceName    string
	ServiceVersion string
}

type metricConfig struct {
	Enabled      bool
	ExporterType string
	ServerPort   int
}

type OotelClient struct {
	traceConfig  *traceConfig
	metricConfig *metricConfig
}

type OotelClientOption func(*OotelClient)

func NewOotelClient(options ...OotelClientOption) *OotelClient {
	client := &OotelClient{}

	// functional options pattern
	for _, option := range options {
		option(client)
	}

	return client
}

func NewTraceConfig(enabled bool, sampleRate float64, serviceName, serviceVersion string) *traceConfig {
	return &traceConfig{
		Enabled:        enabled,
		SampleRate:     sampleRate,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
	}
}

func WithTraceConfig(tc *traceConfig) OotelClientOption {
	return func(oc *OotelClient) {
		oc.traceConfig = tc
	}
}

func NewMetricConfig(enabled bool, exporterType string, serverPort int) *metricConfig {
	return &metricConfig{
		Enabled:      enabled,
		ExporterType: exporterType,
		ServerPort:   serverPort,
	}
}

func WithMetricConfig(mc *metricConfig) OotelClientOption {
	return func(oc *OotelClient) {
		oc.metricConfig = mc
	}
}

func (oc *OotelClient) Init(ctx context.Context) (func(context.Context) error, error) {
	shutdownFuncs := make([]func(context.Context) error, 0)

	shutdown := func(ctx context.Context) error {
		var errs []error
		for _, f := range shutdownFuncs {
			if err := f(ctx); err != nil {
				errs = append(errs, err)
			}
		}

		return errors.Join(errs...)
	}

	if oc.traceConfig != nil && oc.traceConfig.Enabled {
		// Set up propagator.
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		// Set up trace provider.
		tracerProvider, err := traceProvider(ctx, oc.traceConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace provider: %w", err)
		}
		otel.SetTracerProvider(tracerProvider)
	}

	if oc.metricConfig != nil && oc.metricConfig.Enabled {
		// Set up meter provider.
		meterProvider, err := meterProvider(ctx, oc.metricConfig.ExporterType)
		if err != nil {
			return nil, fmt.Errorf("failed to create meter provider: %w", err)
		}
		shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
		otel.SetMeterProvider(meterProvider)

		go func() {
			if err := startServer(oc.metricConfig.ServerPort, oc.metricConfig.ExporterType == "prometheus"); err != nil {
				fmt.Println(err)
			}
		}()
	}

	return shutdown, nil
}

func traceProvider(ctx context.Context, tc *traceConfig) (*trace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	traceResource, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(tc.ServiceName),
			semconv.ServiceVersion(tc.ServiceVersion),
		))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace resource: %w", err)
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(traceExporter)),
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(tc.SampleRate))),
		trace.WithResource(traceResource),
	)
	return traceProvider, nil
}

func meterProvider(ctx context.Context, exporterType string) (*metric.MeterProvider, error) {
	var meterProvider *metric.MeterProvider

	switch exporterType {
	case ExporterTypePrometheus:
		metricExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus metric exporter: %w", err)
		}

		meterProvider = metric.NewMeterProvider(
			metric.WithReader(metricExporter),
		)
	case ExporterTypeOTLPGRPC:
		metricExporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlpgrpc metric exporter: %w", err)
		}

		meterProvider = metric.NewMeterProvider(
			metric.WithReader(
				metric.NewPeriodicReader(metricExporter)),
		)
	case ExporterTypeOTLPHTTP:
		metricExporter, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlphttp metric exporter: %w", err)
		}

		meterProvider = metric.NewMeterProvider(
			metric.WithReader(
				metric.NewPeriodicReader(metricExporter)),
		)
	default:
		return nil, fmt.Errorf("unsupported metric exporter type: %s", exporterType)

	}

	return meterProvider, nil
}

func startServer(port int, isPrometheus bool) error {
	http.HandleFunc("/healthcheck", healthcheck.HealthcheckHandler)
	if isPrometheus {
		http.Handle("/metrics", promhttp.Handler())
	}
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
