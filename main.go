package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	hits := cfg.fileserverHits.Load()
	fmt.Fprintf(w, "Hits: %d", hits)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	fmt.Fprintf(w, "Hits reset to 0")
}

func main() {
	const port = "8080"
	apiCfg := &apiConfig{}

	mux := http.NewServeMux()
	fileServerHandler := http.FileServer(http.Dir("."))
	strippedHandler := http.StripPrefix("/app", fileServerHandler)
	finalHandler := apiCfg.middlewareMetricsInc(strippedHandler)

	mux.Handle("/app/", finalHandler)
	mux.HandleFunc("GET /metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /reset", apiCfg.resetHandler)
	mux.HandleFunc("GET /healthz", readinessHandler)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}
