package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// connectDB — то же, что в api: подключаемся с ретраями.
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
				log.Println("worker connected to postgres")
				return conn
			} else {
				err = pingErr
			}
		}
		log.Printf("worker waiting for postgres (%d/30): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("worker could not connect to postgres: %v", err)
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// processOrders — берёт новые заказы и помечает их выполненными.
func processOrders(db *sql.DB) {
	rows, err := db.Query("SELECT id FROM orders WHERE status = 'new' LIMIT 10")
	if err != nil {
		log.Printf("query failed: %v", err)
		return
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}

	for _, id := range ids {
		_, err := db.Exec("UPDATE orders SET status = 'done' WHERE id = $1", id)
		if err != nil {
			log.Printf("update order %d failed: %v", id, err)
			continue
		}
		log.Printf("order %d marked as done", id)
	}
}

func main() {
	db := connectDB()
	defer db.Close()

	interval := 5 * time.Second
	log.Printf("worker started, polling every %s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		processOrders(db)
	}
}