package main

import (
	"encoding/json"
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
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(200)
	fmt.Fprintf(rw,
		`<html>
  			<body>
    			<h1>Welcome, Chirpy Admin</h1>
   				<p>Chirpy has been visited %d times!</p>
  			</body>
		</html>`, a.fileserverHits.Add(1))
}

func (a *apiConfig) resetHandler(rw http.ResponseWriter, request *http.Request) {
	a.fileserverHits.Store(0)
}

type ChirpyError struct {
	Error string `json:"error"`
}

type ValidateChirpyRequest struct {
	Body string `json:"body"`
}

type ValidChirpyResponse struct {
	Valid bool `json:"valid"`
}

func main() {
	mux := http.ServeMux{}

	apiConfig := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	mux.Handle("/app/", apiConfig.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	mux.HandleFunc("GET /admin/metrics", apiConfig.getRequestCount)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetHandler)

	mux.HandleFunc("GET /api/healthz", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(200)
		response.Write([]byte("OK\n"))
	})

	mux.HandleFunc("POST /api/validate_chirp", func(rw http.ResponseWriter, request *http.Request) {
		var reqBody ValidateChirpyRequest

		decoder := json.NewDecoder(request.Body)
		err := decoder.Decode(&reqBody)

		if err != nil {
			respondWithError(rw, "Something went wrong", 400)
			return
		}
		if len(reqBody.Body) > 140 {
			respondWithError(rw, "Chirp is too long", 400)
			return
		}

		respondWithJSON(rw, ValidChirpyResponse{
			Valid: true,
		})

	})

	server := http.Server{
		Handler: &mux,
		Addr:    ":8080",
	}

	server.ListenAndServe()
}

func respondWithError(rw http.ResponseWriter, errorMessage string, statusCode int) {
	errorResponse, _ := json.Marshal(ChirpyError{
		Error: errorMessage,
	})
	rw.WriteHeader(statusCode)
	rw.Write(errorResponse)
}

func respondWithJSON(rw http.ResponseWriter, data any) {
	v, _ := json.Marshal(data)

	rw.WriteHeader(200)
	rw.Write(v)
}
