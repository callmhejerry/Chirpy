package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (a *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		a.fileserverHits.Add(1)
		next.ServeHTTP(response, request)
	})
}

func (a *apiConfig) getRequestCount(rw http.ResponseWriter, request *http.Request) {
	rw.WriteHeader(200)
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(rw, "Hits: %d\n", a.fileserverHits.Load())
}

func (a *apiConfig) resetHandler(rw http.ResponseWriter, request *http.Request) {
	a.fileserverHits.Store(0)
}

func main() {
	mux := http.ServeMux{}

	apiConfig := apiConfig{}

	mux.Handle("/app/", apiConfig.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("/metrics", apiConfig.getRequestCount)
	mux.HandleFunc("/reset", apiConfig.resetHandler)

	mux.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(200)
		response.Write([]byte("OK"))
	})

	server := http.Server{
		Handler: &mux,
		Addr:    ":8080",
	}

	server.ListenAndServe()
}
