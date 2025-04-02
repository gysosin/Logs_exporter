package main

import (
	"log"
	"net/http"

	"github.com/gysosin/Logs_exporter/internal/collectors"
)

// Simple HTTP server that serves Prometheus-formatted metrics on :9182/metrics
func main() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := collectors.GenerateMetrics()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics))
	})

	addr := ":9182"
	log.Printf("Listening on %s (Press Ctrl+C to stop)", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
