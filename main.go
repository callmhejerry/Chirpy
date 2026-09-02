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

	"github.com/callmhejerry/Chirpy/internal/auth"
	"github.com/callmhejerry/Chirpy/internal/database"
	"github.com/callmhejerry/Chirpy/internal/requests"
	"github.com/callmhejerry/Chirpy/internal/responses"
	"github.com/callmhejerry/Chirpy/internal/utils"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	DB             *database.Queries
	Platform       string
	Secret         string
	PolkaApiKey    string
}

func (a *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		a.fileserverHits.Add(1)
		next.ServeHTTP(response, request)
	})
}

func (a *apiConfig) middlewareAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, request *http.Request) {
		accessToken, err := auth.GetBearerToken(request.Header)
		if err != nil {
			respondWithError(rw, "Unauthorized", 401, err)
			return
		}
		userId, err := auth.ValidateJwt(accessToken, a.Secret)
		if err != nil {
			respondWithError(rw, "Unauthorized", 401, err)
			return
		}

		request.Header.Set("X-User-ID", userId.String())
		next.ServeHTTP(rw, request)
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
		respondWithError(rw, "Unauthorized", 403, nil)
	}
	a.fileserverHits.Store(0)
	a.DB.DeleteAllUsers(request.Context())
}
func (a *apiConfig) createUserHandler(rw http.ResponseWriter, request *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	jsonDecoder := json.NewDecoder(request.Body)

	if err := jsonDecoder.Decode(&body); err != nil {
		respondWithError(rw, "Failed to parse json body", 400, err)
		return
	}
	hashedPassword, err := auth.HashPassword(body.Password)

	if err != nil {
		respondWithError(rw, "Something went wrong, failed to hash password", 500, err)
		return
	}
	user, err := a.DB.CreateUser(request.Context(), database.CreateUserParams{
		Email:          body.Email,
		HashedPassword: hashedPassword,
	})

	if err != nil {

		respondWithError(rw, "failed to create user in database", 400, err)
		return
	}

	respondWithJSON(rw, responses.UserResponseModel{
		Id:          user.ID.String(),
		Email:       user.Email,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		IsChirpyRed: user.IsChirpyRed,
	})
}

func (a *apiConfig) createChirpHandler(rw http.ResponseWriter, request *http.Request) {
	token, err := auth.GetBearerToken(request.Header)

	if err != nil {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}
	userId, err := auth.ValidateJwt(token, a.Secret)

	if err != nil {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	var jsonBody struct {
		Body string `json:"body"`
	}

	jsonDecoder := json.NewDecoder(request.Body)
	if err := jsonDecoder.Decode(&jsonBody); err != nil {
		respondWithError(rw, "Failed to decode request", 400, err)
		return
	}

	chirp, err := a.DB.CreateChirp(request.Context(), database.CreateChirpParams{
		Body:   jsonBody.Body,
		UserID: userId,
	})
	if err != nil {
		respondWithError(rw, "Failed to create chirp in the database", 400, err)
		return
	}
	rw.WriteHeader(201)
	respondWithJSON(rw, responses.ChirpResponseModel{
		Id:        chirp.ID.String(),
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserId:    chirp.UserID.String(),
		Body:      chirp.Body,
	})
}

func (a *apiConfig) getAllChirps(rw http.ResponseWriter, request *http.Request) {
	allChirps, err := a.DB.GetAllChirps(request.Context())
	if err != nil {
		respondWithError(rw, "Failed to get all chirps from database", 400, err)
		return
	}
	chirpResponse := make([]responses.ChirpResponseModel, len(allChirps))

	for _, chirp := range allChirps {
		chirpResponse = append(chirpResponse, responses.ChirpResponseModel{
			Id:        chirp.ID.String(),
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			UserId:    chirp.ID.String(),
			Body:      chirp.Body,
		})
	}
	respondWithJSON(rw, chirpResponse)
}

func (a *apiConfig) getChirpById(rw http.ResponseWriter, request *http.Request) {
	chirpId, err := uuid.Parse(request.PathValue("chirpId"))

	if err != nil {
		respondWithError(rw, "Invalid chirp ID", 400, err)
		return
	}

	chirp, err := a.DB.GetChirpById(request.Context(), chirpId)
	if err != nil {
		respondWithError(rw, "Failed to get chirp from Database", 404, err)
		return
	}
	respondWithJSON(rw, responses.ChirpResponseModel{
		Id:        chirpId.String(),
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserId:    chirp.UserID.String(),
		Body:      chirp.Body,
	})
}

func (a *apiConfig) loginUserHandler(rw http.ResponseWriter, request *http.Request) {
	requestBody, err := utils.ParseRequestBody[requests.LoginRequest](request.Body)
	if err != nil {
		respondWithError(rw, "Invalid request", 400, err)
		return
	}

	email := requestBody.Email
	password := requestBody.Password
	expiresIn := 1 * time.Hour

	user, err := a.DB.GetUserByEmail(request.Context(), email)

	if err != nil {
		respondWithError(rw, "Incorrect email or password", 401, err)
		return
	}

	passwordMatched, err := auth.CheckPasswordHash(password, user.HashedPassword)

	if err != nil {
		respondWithError(rw, "Failed to check hashed password", 500, err)
		return
	}

	if !passwordMatched {
		respondWithError(rw, "Incorrect email or password", 401, password)
		return
	}

	token, err := auth.MakeJWT(user.ID, a.Secret, expiresIn)

	if err != nil {
		respondWithError(rw, "Failed to generate JWT token", 500, err)
		return
	}

	refreshToken := auth.MakeRefreshToken()

	createdRefreshToken, err := a.DB.CreateRefeshToken(request.Context(), database.CreateRefeshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(60 * (24 * time.Hour)),
	})

	if err != nil {
		respondWithError(rw, "Failed to create refresh token in database", 500, err)
		return
	}

	respondWithJSON(rw, responses.UserResponseModel{
		Id:           user.ID.String(),
		Email:        user.Email,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Token:        token,
		RefreshToken: createdRefreshToken.Token,
	})
}
func (a *apiConfig) refreshTokenHandler(rw http.ResponseWriter, request *http.Request) {
	refreshToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}
	createdRefreshToken, err := a.DB.GetRefreshToken(request.Context(), refreshToken)

	if err != nil {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	if createdRefreshToken.RevokedAt.Valid || createdRefreshToken.ExpiresAt.Before(time.Now()) {
		fmt.Printf("Refresh token is revoked or expired: %v\n", createdRefreshToken)
		fmt.Printf("Current time: %v\n", time.Now())
		fmt.Printf("Refresh token expiration time: %v\n", createdRefreshToken.ExpiresAt)
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	newRefreshToken := auth.MakeRefreshToken()

	a.DB.UpdateRefreshToken(request.Context(), database.UpdateRefreshTokenParams{
		UserID:    createdRefreshToken.UserID,
		Token:     newRefreshToken,
		ExpiresAt: time.Now().Add(60 * (24 * time.Hour)),
	})

	respondWithJSON(rw, struct {
		Token string `json:"token"`
	}{Token: newRefreshToken})
}

func (a *apiConfig) revokeRefreshTokenHandler(rw http.ResponseWriter, request *http.Request) {
	refreshToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	createdRefreshToken, err := a.DB.GetRefreshToken(request.Context(), refreshToken)

	if err != nil {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	if createdRefreshToken.RevokedAt.Valid || createdRefreshToken.ExpiresAt.Before(time.Now()) {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	a.DB.RevokeRefreshToken(request.Context(), createdRefreshToken.Token)

	rw.WriteHeader(204)
}

func (a *apiConfig) updateUserHandler(rw http.ResponseWriter, request *http.Request) {
	requestBody, err := utils.ParseRequestBody[requests.LoginRequest](request.Body)
	if err != nil {
		respondWithError(rw, "Invalid request", 400, err)
		return
	}

	userId := request.Header.Get("X-User-ID")
	hashedPassword, err := auth.HashPassword(requestBody.Password)

	if err != nil {
		respondWithError(rw, "Something went wrong, failed to hash password", 500, err)
		return
	}

	newUser, err := a.DB.UpdateUser(request.Context(), database.UpdateUserParams{
		ID:             uuid.MustParse(userId),
		Email:          requestBody.Email,
		HashedPassword: hashedPassword,
	})

	if err != nil {
		respondWithError(rw, "failed to update user in database", 400, err)
		return
	}

	respondWithJSON(rw, struct {
		Id          string    `json:"id"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
		Email       string    `json:"email"`
		IsChirpyRed bool      `json:"is_chirpy_red"`
	}{
		Email:       newUser.Email,
		Id:          newUser.ID.String(),
		CreatedAt:   newUser.CreatedAt,
		UpdatedAt:   newUser.UpdatedAt,
		IsChirpyRed: newUser.IsChirpyRed,
	})

}

func (a *apiConfig) deleteChirpHandler(rw http.ResponseWriter, request *http.Request) {
	chirpId, err := uuid.Parse(request.PathValue("chirpId"))

	if err != nil {
		respondWithError(rw, "Invalid chirp ID", 400, err)
		return
	}
	userId, err := uuid.Parse(request.Header.Get("X-User-ID"))
	if err != nil {
		respondWithError(rw, "Invalid user ID", 400, err)
		return
	}

	chirp, err := a.DB.GetChirpById(request.Context(), chirpId)
	if err != nil {
		respondWithError(rw, "Chirp not found", 404, err)
		return
	}

	if chirp.UserID != userId {
		respondWithError(rw, "Unauthorized", 403, err)
		return
	}

	if err := a.DB.DeleteChirpById(request.Context(), database.DeleteChirpByIdParams{
		ID:     chirpId,
		UserID: userId,
	}); err != nil {
		respondWithError(rw, "Unauthorized", 403, err)
		return
	}

	rw.WriteHeader(204)
}

func (a *apiConfig) polkaWebhookHandler(rw http.ResponseWriter, request *http.Request) {
	apiKey, err := auth.GetApiKey(request.Header)

	if err != nil {
		respondWithError(rw, err.Error(), 401, err)
		return
	}
	if apiKey != a.PolkaApiKey {
		respondWithError(rw, "Unauthorized", 401, err)
		return
	}

	requestBody, err := utils.ParseRequestBody[struct {
		Event string `json:"event"`
		Data  struct {
			UserId string `json:"user_id"`
		} `json:"data"`
	}](request.Body)

	if err != nil {
		respondWithError(rw, "Invalid request", 400, err)
		return
	}

	if requestBody.Event != "user.upgraded" {
		rw.WriteHeader(204)
		return
	}

	userId, err := uuid.Parse(requestBody.Data.UserId)

	if err != nil {
		respondWithError(rw, "Invalid user id", 400, err)
		return
	}

	if _, err := a.DB.UpdateUserMembership(request.Context(), database.UpdateUserMembershipParams{
		ID:          userId,
		IsChirpyRed: true,
	}); err != nil {
		respondWithError(rw, "User cannot be found", 404, err)
		return
	}

	rw.WriteHeader(204)
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
	secret := os.Getenv("SECRET")
	polkaApiKey := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		log.Fatalf("Failed to establish a connection to database %v", err)
	}

	mux := http.ServeMux{}

	apiConfig := apiConfig{
		fileserverHits: atomic.Int32{},
		DB:             database.New(db),
		Platform:       platform,
		Secret:         secret,
		PolkaApiKey:    polkaApiKey,
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
			respondWithError(rw, "Something went wrong", 400, err)
			return
		}
		if len(reqBody.Body) > 140 {
			respondWithError(rw, "Chirp is too long", 400, err)
			return
		}

		respondWithJSON(rw, struct {
			CleanedBody string `json:"cleaned_body"`
		}{
			CleanedBody: maskBadWords(reqBody.Body),
		})

	})

	mux.HandleFunc("POST /api/users", apiConfig.createUserHandler)
	mux.HandleFunc("POST /api/chirps", apiConfig.createChirpHandler)
	mux.HandleFunc("GET /api/chirps", apiConfig.getAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpId}", apiConfig.getChirpById)
	mux.HandleFunc("POST /api/login", apiConfig.loginUserHandler)
	mux.HandleFunc("POST /api/refresh", apiConfig.refreshTokenHandler)
	mux.HandleFunc("POST /api/revoke", apiConfig.revokeRefreshTokenHandler)
	mux.Handle("PUT /api/users", apiConfig.middlewareAuth(apiConfig.updateUserHandler))
	mux.Handle("DELETE /api/chirps/{chirpId}", apiConfig.middlewareAuth(apiConfig.deleteChirpHandler))
	mux.HandleFunc("PUT /api/polka/webhooks", apiConfig.polkaWebhookHandler)

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

func respondWithError(rw http.ResponseWriter, errorMessage string, statusCode int, err any) {
	fmt.Printf("ERROR : %v\n", err)
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
