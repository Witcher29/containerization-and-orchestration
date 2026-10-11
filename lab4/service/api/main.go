package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/lib/pq"
)

var db *sql.DB

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled by the API.",
		},
		[]string{"method", "path", "code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed.",
		},
	)

	ordersCreatedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "shop_orders_created_total",
			Help: "Total number of successfully created orders.",
		},
	)
)

func initMetrics() {
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		httpRequestsInFlight,
		ordersCreatedTotal,
	)
}

// metricsMiddleware records request count, status code, and duration.
// The /metrics endpoint is excluded to avoid monitoring noise.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start).Seconds()
		code := strconv.Itoa(recorder.statusCode)

		httpRequestsTotal.WithLabelValues(
			r.Method,
			r.URL.Path,
			code,
		).Inc()

		httpRequestDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
		).Observe(duration)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.statusCode = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(data)
}

// Health — health-check endpoint.
// HEALTH_FAIL=true returns HTTP 500 for failure testing.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("HEALTH_FAIL") == "true" {
		http.Error(w, "unhealthy", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// POST /order creates a new order in PostgreSQL.
func orderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var id int
	err := db.QueryRow(
		"INSERT INTO orders (status) VALUES ($1) RETURNING id",
		"new",
	).Scan(&id)
	if err != nil {
		log.Printf("insert order failed: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	ordersCreatedTotal.Inc()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":     id,
		"status": "new",
	})
}

// GET /orders returns all orders.
func ordersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, status FROM orders ORDER BY id")
	if err != nil {
		log.Printf("query orders failed: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Order struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	}

	orders := make([]Order, 0)
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Status); err != nil {
			log.Printf("scan order failed: %v", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		log.Printf("iterate orders failed: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// connectDB retries until PostgreSQL is ready.
func connectDB() *sql.DB {
	host := env("POSTGRES_HOST", "postgres")
	port := env("POSTGRES_PORT", "5432")
	user := env("POSTGRES_USER", "shop")
	pass := env("POSTGRES_PASSWORD", "shop")
	name := env("POSTGRES_DB", "shop")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, name,
	)

	var conn *sql.DB
	var err error

	for i := 0; i < 30; i++ {
		conn, err = sql.Open("postgres", dsn)
		if err == nil {
			if pingErr := conn.Ping(); pingErr == nil {
				log.Println("connected to postgres")
				return conn
			} else {
				err = pingErr
				conn.Close()
			}
		}

		log.Printf("waiting for postgres (%d/30): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("could not connect to postgres: %v", err)
	return nil
}

func env(key, def string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return def
}

func main() {
	initMetrics()

	db = connectDB()
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/order", orderHandler)
	mux.HandleFunc("/orders", ordersHandler)
	mux.Handle("/metrics", promhttp.Handler())

	addr := ":" + env("PORT", "8080")
	handler := metricsMiddleware(mux)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
