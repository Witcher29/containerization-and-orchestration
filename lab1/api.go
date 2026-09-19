package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var memoryBallast []byte

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "ok")
}

func eat(w http.ResponseWriter, r *http.Request) {
	mb := r.URL.Query().Get("mb")
	if mb == "" {
		http.Error(w, "missing mb", 400)
		return
	}
	var n int
	fmt.Sscanf(mb, "%d", &n)
	// Выделяем n мегабайт
	memoryBallast = make([]byte, n*1024*1024)
	// Заполняем, чтобы память реально была занята
	for i := range memoryBallast {
		memoryBallast[i] = 1
	}
	fmt.Fprintf(w, "allocated %d MB\n", n)
}

func burn(w http.ResponseWriter, r *http.Request) {
	// Нагружаем одно ядро
	go func() {
		for {
			// бесконечный цикл
		}
	}()
	fmt.Fprint(w, "burning\n")
}

func main() {
	http.HandleFunc("/health", health)
	http.HandleFunc("/eat", eat)
	http.HandleFunc("/burn", burn)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}