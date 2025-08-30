package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AlanZeng-Coder/linkwatch/internal/api"
	"github.com/AlanZeng-Coder/linkwatch/internal/storage"

	"github.com/AlanZeng-Coder/linkwatch/internal/checker"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	checkInterval := getEnvDuration("CHECK_INTERVAL", 15*time.Second)
	maxConc := getEnvInt("MAX_CONCURRENCY", 8)
	httpTimeout := getEnvDuration("HTTP_TIMEOUT", 5*time.Second)
	shutdownGrace := getEnvDuration("SHUTDOWN_GRACE", 10*time.Second)

	db, err := sql.Open("sqlite3", "./linkwatch.db")
	if err != nil {
		log.Fatal(err)
	}
	s := storage.NewSQLiteStorage(db)
	if err := s.Init(context.Background()); err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	c := checker.NewChecker(s, checkInterval, maxConc, httpTimeout)
	go c.Start()

	h := api.NewHandler(s)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/targets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			h.PostTarget(w, r)
		} else if r.Method == "GET" {
			h.ListTargets(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/targets/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			path := r.URL.Path
			if strings.HasSuffix(path, "/results") {
				id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/targets/"), "/results")
				h.GetResults(w, r, id)
			} else {
				h.ListTargets(w, r)
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	c.Stop()
	srv.Shutdown(ctx)
	log.Println("Shutdown complete")
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
