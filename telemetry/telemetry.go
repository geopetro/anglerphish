// Package telemetry adds OpenTelemetry (OTel) telemetry to Gophish:
// distributed tracing, metrics and bridged logs. Providers are registered
// with the global OTel API so any otelhttp-instrumented handler can produce
// spans. Configuration comes from the telemetry section of config.json and
// the OTEL_EXPORTER_OTLP_ENDPOINT environment variable.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/logger"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.43.0"
)

// DefaultServiceName is used when the config has no service_name set.
const DefaultServiceName = "gophish"

// metricExportInterval is how often metrics are flushed to the collector.
const metricExportInterval = 15 * time.Second

var shutdownFuncs []func(context.Context) error

// Setup initializes the OTel tracer, meter and log providers based on the
// given telemetry config. When disabled, no providers are registered and the
// global no-op providers are used. Logs are bridged to the logger regardless,
// so OTel log records are produced even without an OTLP endpoint configured.
func Setup(cfg *config.Telemetry) error {
	if cfg == nil {
		cfg = &config.Telemetry{Enable: true}
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = DefaultServiceName
	}
	if !cfg.Enable {
		BridgeLogrus()
		return nil
	}
	if err := setupTraces(cfg); err != nil {
		return fmt.Errorf("telemetry: traces: %w", err)
	}
	if err := setupMetrics(cfg); err != nil {
		return fmt.Errorf("telemetry: metrics: %w", err)
	}
	if err := setupLogs(cfg); err != nil {
		return fmt.Errorf("telemetry: logs: %w", err)
	}
	BridgeLogrus()
	logger.Infof("OTel telemetry enabled (service_name=%s, otlp_endpoint=%q)",
		cfg.ServiceName, cfg.OTLPEndpoint)
	return nil
}

// Shutdown flushes all telemetry providers. Call it on graceful shutdown.
func Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(shutdownFuncs) - 1; i >= 0; i-- {
		if err := shutdownFuncs[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	shutdownFuncs = nil
	if len(errs) == 1 {
		return errs[0]
	}
	if len(errs) > 1 {
		return fmt.Errorf("telemetry shutdown errors: %v", errs)
	}
	return nil
}

func endpoint(cfg *config.Telemetry) string {
	if cfg.OTLPEndpoint != "" {
		return cfg.OTLPEndpoint
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

func setupTraces(cfg *config.Telemetry) error {
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(serviceResource(cfg.ServiceName)))
	otelapi.SetTracerProvider(tp)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	return nil
}

func setupMetrics(cfg *config.Telemetry) error {
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(serviceResource(cfg.ServiceName)),
	}
	if !cfg.DisableMetrics {
		if ep := endpoint(cfg); ep != "" {
			exp, err := otlpmetricgrpc.New(context.Background(),
				otlpmetricgrpc.WithEndpoint(ep), otlpmetricgrpc.WithInsecure())
			if err != nil {
				return err
			}
			opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
				sdkmetric.WithInterval(metricExportInterval))))
		}
	}
	mp := sdkmetric.NewMeterProvider(opts...)
	otelapi.SetMeterProvider(mp)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	return nil
}

func setupLogs(cfg *config.Telemetry) error {
	ep := endpoint(cfg)
	if ep == "" {
		return nil
	}
	exp, err := otlploggrpc.New(context.Background(),
		otlploggrpc.WithEndpoint(ep), otlploggrpc.WithInsecure())
	if err != nil {
		return err
	}
	lp := log.NewLoggerProvider(
		log.WithResource(serviceResource(cfg.ServiceName)),
		log.WithProcessor(log.NewBatchProcessor(exp)),
	)
	otelapi.SetLoggerProvider(lp)
	shutdownFuncs = append(shutdownFuncs, lp.Shutdown)
	return nil
}

func serviceResource(name string) *resource.Resource {
	return resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceNameKey.String(name))
}
