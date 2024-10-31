package main

import (
	"encoding/json"
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
	w.Header().Set("Content-Type", "text/html")
	html := fmt.Sprintf(`
	<html>
		<body>
			<h1>Welcome, Chirpy Admin </h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>
	`, hits)
	w.Write([]byte(html))
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/html")
	html := (`
	<html>
		<body>
			<h1>Welcome, Chirpy Admin </h1>
			<p>Hits reset to 0"</p>
		</body>
	</html>
	`)
	w.Write([]byte(html))
}

func (cfg *apiConfig) validateHandler(w http.ResponseWriter, r *http.Request) {
	type returnVals struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	rvals := returnVals{}

	err := decoder.Decode(&rvals)
	if err != nil {
		errResponse := struct {
			Error string `json:"error"`
		}{
			Error: "Something went wrong",
		}

		jsonData, marshalErr := json.Marshal(errResponse)
		if marshalErr != nil {
			log.Printf("Error marshalling errResponse: %s", marshalErr)
			w.WriteHeader(500)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}

	if len(rvals.Body) > 140 {
		tooLongResponse := struct {
			Error string `json:"error"`
		}{
			Error: "Chirp is too long",
		}

		jsonData, marshalErr := json.Marshal(tooLongResponse)
		if marshalErr != nil {
			log.Printf("Error marshalling errResponse: %s", marshalErr)
			w.WriteHeader(400)
			w.Write([]byte("Internal Server Error"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(jsonData)
		return
	}

	validResponse := struct {
		Valid bool `json:"valid"`
	}{
		Valid: true,
	}

	jsonData, marshalErr := json.Marshal(validResponse)
	if marshalErr != nil {
		log.Printf("Error marshalling errResponse: %s", err)
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(jsonData)
}

func main() {
	const port = "8080"
	apiCfg := &apiConfig{}

	mux := http.NewServeMux()
	fileServerHandler := http.FileServer(http.Dir("."))
	strippedHandler := http.StripPrefix("/app", fileServerHandler)
	finalHandler := apiCfg.middlewareMetricsInc(strippedHandler)

	mux.Handle("/app/", finalHandler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /api/validate_chirp", apiCfg.validateHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)
	mux.HandleFunc("GET /api/healthz", readinessHandler)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}
