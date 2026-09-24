package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// ---------- Метрики (RED) ----------
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
			Help: "Total number of HTTP errors (5xx).",
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
	// Счётчик ошибок, увеличиваемый эндпоинтом /fail
	failCounter atomic.Int64
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpErrorsTotal, httpRequestDuration)
}

// ---------- OpenTelemetry ----------
func initTracer() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("localhost:4318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName("api-service"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp, nil
}

// ---------- JSON-логгер с trace_id ----------
type traceIDHandler struct {
	slog.Handler
}

func (h *traceIDHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		r.AddAttrs(slog.String("trace_id", span.SpanContext().TraceID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

// ---------- Middleware ----------
func instrument(next http.Handler) http.Handler {
	tracer := otel.Tracer("api-service")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем входящий trace-контекст
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.URL.Path)
		defer span.End()

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))
		duration := time.Since(start).Seconds()

		route := r.URL.Path
		httpRequestsTotal.WithLabelValues(r.Method, route, fmt.Sprint(rw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(duration)
		if rw.status >= 500 {
			httpErrorsTotal.WithLabelValues(r.Method, route).Inc()
		}

		span.SetAttributes(
			attribute.Int("http.status_code", rw.status),
			attribute.Float64("http.duration_seconds", duration),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// ---------- Хендлеры ----------
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func failHandler(w http.ResponseWriter, r *http.Request) {
	failCounter.Add(1)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	delay := time.Duration(1+rand.Intn(3)) * time.Second
	time.Sleep(delay)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "slept for %v", delay)
}

func loadHandler(w http.ResponseWriter, r *http.Request) {
	count := 20
	client := &http.Client{Timeout: 5 * time.Second}
	var success int64
	for i := 0; i < count; i++ {
		resp, err := client.Get("http://localhost:8080/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&success, 1)
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "generated %d requests, %d successful", count, success)
}

// ---------- main ----------
func main() {
	// Логгер
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(&traceIDHandler{Handler: baseHandler})
	slog.SetDefault(logger)

	// OpenTelemetry
	tp, err := initTracer()
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Маршруты
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/fail", failHandler)
	mux.HandleFunc("/slow", slowHandler)
	mux.HandleFunc("/load", loadHandler)
	mux.Handle("/metrics", promhttp.Handler())

	// Обёртка инструментирования
	handler := instrument(mux)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	slog.Info("starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// ---------- вспомогательный JSON-ответ ----------
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}