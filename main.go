package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/callmhejerry/Chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	DB             *database.Queries
	Platform       string
}

type UserResponseModel struct {
	Id        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	if a.Platform != "dev" {
		respondWithError(rw, "Unauthorized", 403)
	}
	a.fileserverHits.Store(0)
	a.DB.DeleteAllUsers(request.Context())
}

func (a *apiConfig) createUserHandler(rw http.ResponseWriter, request *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	jsonDecoder := json.NewDecoder(request.Body)

	if err := jsonDecoder.Decode(&body); err != nil {
		respondWithError(rw, "Failed to parse json body", 400)
		return
	}

	user, err := a.DB.CreateUser(request.Context(), body.Email)

	if err != nil {
		respondWithError(rw, "failed to create user in database", 400)
		return
	}

	respondWithJSON(rw, UserResponseModel{
		Id:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	})
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
	godotenv.Load()

	platform := os.Getenv("PLATFORM")
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		log.Fatalf("Failed to establish a connection to database %v", err)
	}

	mux := http.ServeMux{}

	apiConfig := apiConfig{
		fileserverHits: atomic.Int32{},
		DB:             database.New(db),
		Platform:       platform,
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

		respondWithJSON(rw, struct {
			CleanedBody string `json:"cleaned_body"`
		}{
			CleanedBody: maskBadWords(reqBody.Body),
		})

	})

	mux.HandleFunc("POST /api/users", apiConfig.createUserHandler)

	server := http.Server{
		Handler: &mux,
		Addr:    ":8080",
	}

	server.ListenAndServe()
}

func maskBadWords(content string) string {
	badWords := [3]string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Fields(content)

	for i, word := range words {
		if slices.Contains(badWords[:], strings.ToLower(word)) {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
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
