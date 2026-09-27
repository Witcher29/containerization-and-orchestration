package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultPort              = "8080"
	defaultOTLPEndpoint      = "jaeger:4318"
	defaultSelfURL           = "http://api:8080"
	defaultLoadRequests      = 20
	defaultHTTPClientTimeout = 5 * time.Second
)

// ============================================================
// Configuration
// ============================================================

type Config struct {
	Port              string
	OTLPEndpoint      string
	SelfURL           string
	LoadRequests      int
	HTTPClientTimeout time.Duration
}

func loadConfig() Config {
	return Config{
		Port:              getEnv("PORT", defaultPort),
		OTLPEndpoint:      getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", defaultOTLPEndpoint),
		SelfURL:           getEnv("SELF_URL", defaultSelfURL),
		LoadRequests:      getEnvInt("LOAD_REQUESTS", defaultLoadRequests),
		HTTPClientTimeout: getEnvDuration("HTTP_CLIENT_TIMEOUT", defaultHTTPClientTimeout),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		slog.Warn(
			"invalid integer environment variable, using default",
			"key", key,
			"value", value,
			"default", fallback,
			"error", err,
		)
		return fallback
	}

	return result
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		slog.Warn(
			"invalid duration environment variable, using default",
			"key", key,
			"value", value,
			"default", fallback,
			"error", err,
		)
		return fallback
	}

	return result
}

// ============================================================
// Prometheus metrics
// ============================================================

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "code"},
	)

	httpErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP 5xx responses.",
		},
		[]string{"method", "route"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpErrorsTotal,
		httpRequestDuration,
	)
}

// ============================================================
// OpenTelemetry
// ============================================================

func initTracer(endpoint string) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	exporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	resource, err := sdkresource.New(
		ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName("api-service"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

// ============================================================
// JSON logger with trace_id
// ============================================================

type traceIDHandler struct {
	slog.Handler
}

func (h *traceIDHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)

	if span.SpanContext().IsValid() {
		record.AddAttrs(
			slog.String(
				"trace_id",
				span.SpanContext().TraceID().String(),
			),
		)

		record.AddAttrs(
			slog.String(
				"span_id",
				span.SpanContext().SpanID().String(),
			),
		)
	}

	return h.Handler.Handle(ctx, record)
}

// ============================================================
// HTTP instrumentation middleware
// ============================================================

func instrument(next http.Handler) http.Handler {
	tracer := otel.Tracer("api-service")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming HTTP headers.
		ctx := otel.GetTextMapPropagator().Extract(
			r.Context(),
			propagation.HeaderCarrier(r.Header),
		)

		// Create root/server span.
		ctx, span := tracer.Start(
			ctx,
			r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// Pass the context containing the span to the handler.
		next.ServeHTTP(rw, r.WithContext(ctx))

		duration := time.Since(start)

		route := r.URL.Path
		statusCode := rw.status

		// Prometheus RED metrics.
		httpRequestsTotal.
			WithLabelValues(
				r.Method,
				route,
				strconv.Itoa(statusCode),
			).
			Inc()

		httpRequestDuration.
			WithLabelValues(
				r.Method,
				route,
			).
			Observe(duration.Seconds())

		if statusCode >= 500 {
			httpErrorsTotal.
				WithLabelValues(
					r.Method,
					route,
				).
				Inc()

			span.SetStatus(
				codes.Error,
				fmt.Sprintf("HTTP %d", statusCode),
			)
		}

		// OpenTelemetry HTTP attributes.
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", statusCode),
			attribute.Float64(
				"http.duration_seconds",
				duration.Seconds(),
			),
		)

		// Structured request log.
		slog.InfoContext(
			ctx,
			"http request",
			"method", r.Method,
			"route", route,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
		)
	})
}

// ============================================================
// Response writer
// ============================================================

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.wroteHeader = true

	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(body []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	return rw.ResponseWriter.Write(body)
}

// ============================================================
// Handlers
// ============================================================

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func failHandler(w http.ResponseWriter, r *http.Request) {
	// Get current request span.
	span := trace.SpanFromContext(r.Context())

	// Mark the span as failed.
	span.SetStatus(
		codes.Error,
		"internal server error",
	)

	span.SetAttributes(
		attribute.String("error.type", "internal_server_error"),
	)

	// This log contains trace_id automatically.
	slog.ErrorContext(
		r.Context(),
		"request failed",
		"reason", "simulated internal server error",
	)

	http.Error(
		w,
		"internal server error",
		http.StatusInternalServerError,
	)
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("api-service")

	// Create nested span.
	ctx, span := tracer.Start(
		r.Context(),
		"slow-op",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	delay := time.Duration(1+rand.Intn(3)) * time.Second

	span.SetAttributes(
		attribute.Int64(
			"sleep.duration_ms",
			delay.Milliseconds(),
		),
	)

	slog.InfoContext(
		ctx,
		"starting slow operation",
		"delay_ms", delay.Milliseconds(),
	)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Normal completion.

	case <-ctx.Done():
		span.SetStatus(
			codes.Error,
			"request cancelled",
		)

		slog.WarnContext(
			ctx,
			"slow operation cancelled",
		)

		return
	}

	slog.InfoContext(
		ctx,
		"slow operation completed",
	)

	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(
		w,
		"slept for %v",
		delay,
	)
}

// ============================================================
// Load generator
// ============================================================

func loadHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := &http.Client{
			Timeout: cfg.HTTPClientTimeout,
		}

		targetURL := cfg.SelfURL + "/health"

		var success int64

		for i := 0; i < cfg.LoadRequests; i++ {
			// Create child span for each outgoing request.
			tracer := otel.Tracer("api-service")

			ctx, span := tracer.Start(
				r.Context(),
				"load-request",
				trace.WithSpanKind(trace.SpanKindClient),
			)

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				targetURL,
				nil,
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to create request")
				span.End()
				continue
			}

			// Propagate trace context to the /health request.
			otel.GetTextMapPropagator().Inject(
				ctx,
				propagation.HeaderCarrier(req.Header),
			)

			resp, err := client.Do(req)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(
					codes.Error,
					"health request failed",
				)
				span.End()
				continue
			}

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&success, 1)
			} else {
				span.SetStatus(
					codes.Error,
					fmt.Sprintf(
						"health request returned HTTP %d",
						resp.StatusCode,
					),
				)
			}

			_, _ = resp.Body.Read(make([]byte, 1))
			_ = resp.Body.Close()

			span.SetAttributes(
				attribute.Int(
					"http.status_code",
					resp.StatusCode,
				),
			)

			span.End()
		}

		slog.InfoContext(
			r.Context(),
			"load generated",
			"requests", cfg.LoadRequests,
			"successful", success,
			"target", targetURL,
		)

		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(
			w,
			"generated %d requests, %d successful",
			cfg.LoadRequests,
			success,
		)
	}
}

// ============================================================
// Main
// ============================================================

func main() {
	// ----------------------------------------------------------
	// Logger
	// ----------------------------------------------------------

	baseHandler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	)

	logger := slog.New(
		&traceIDHandler{
			Handler: baseHandler,
		},
	)

	slog.SetDefault(logger)

	// ----------------------------------------------------------
	// Configuration
	// ----------------------------------------------------------

	cfg := loadConfig()

	slog.Info(
		"configuration loaded",
		"port", cfg.Port,
		"otlp_endpoint", cfg.OTLPEndpoint,
		"self_url", cfg.SelfURL,
		"load_requests", cfg.LoadRequests,
	)

	// ----------------------------------------------------------
	// OpenTelemetry
	// ----------------------------------------------------------

	tp, err := initTracer(cfg.OTLPEndpoint)
	if err != nil {
		slog.Error(
			"failed to initialize OpenTelemetry",
			"error", err,
		)
		os.Exit(1)
	}

	// ----------------------------------------------------------
	// HTTP routes
	// ----------------------------------------------------------

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/health",
		healthHandler,
	)

	mux.HandleFunc(
		"/fail",
		failHandler,
	)

	mux.HandleFunc(
		"/slow",
		slowHandler,
	)

	mux.HandleFunc(
		"/load",
		loadHandler(cfg),
	)

	mux.Handle(
		"/metrics",
		promhttp.Handler(),
	)

	// ----------------------------------------------------------
	// HTTP server
	// ----------------------------------------------------------

	handler := instrument(mux)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// ----------------------------------------------------------
	// Graceful shutdown
	// ----------------------------------------------------------

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		<-signalCtx.Done()

		slog.Info("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error(
				"HTTP server shutdown error",
				"error", err,
			)
		}

		if err := tp.Shutdown(shutdownCtx); err != nil {
			slog.Error(
				"OpenTelemetry shutdown error",
				"error", err,
			)
		}

		slog.Info("shutdown completed")
	}()

	// ----------------------------------------------------------
	// Start
	// ----------------------------------------------------------

	slog.Info(
		"starting HTTP server",
		"addr", server.Addr,
	)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {

		slog.Error(
			"HTTP server error",
			"error", err,
		)

		os.Exit(1)
	}
}
