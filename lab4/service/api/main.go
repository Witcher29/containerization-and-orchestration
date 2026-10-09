package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

// Health — health-check endpoint.
// Если HEALTH_FAIL=true, возвращаем 500 — это используется
// в части 2 для проверки rolling update с неисправной версией.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("HEALTH_FAIL") == "true" {
		http.Error(w, "unhealthy", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// Order — POST /order: создаёт новый заказ в Postgres.
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "new"})
}

// Orders — GET /orders: возвращает список заказов.
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
	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Status); err != nil {
			continue
		}
		orders = append(orders, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// connectDB — подключается к Postgres с ретраями.
// БД может ещё не быть готова на старте пода,
// поэтому ждём и пробуем снова.
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
			}
		}
		log.Printf("waiting for postgres (%d/30): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("could not connect to postgres: %v", err)
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	db = connectDB()
	defer db.Close()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/order", orderHandler)
	http.HandleFunc("/orders", ordersHandler)

	addr := ":" + env("PORT", "8080")
	log.Printf("api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}