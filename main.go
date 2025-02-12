package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/krishnaravi94/Chirpy/internal/auth"
	"github.com/krishnaravi94/Chirpy/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	database       *database.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(response, request)
	})
}

func (cfg *apiConfig) fetchMetrics(response http.ResponseWriter, request *http.Request) {
	hits := cfg.fileserverHits.Load()
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(200)
	hitsString := fmt.Sprintf(`<html>

<body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
</body>

</html>`, hits)
	response.Write([]byte(hitsString))
}

func (cfg *apiConfig) resetMetrics(response http.ResponseWriter, request *http.Request) {
	cfg.fileserverHits.Store(0)
	resetError := cfg.database.ResetDatabase(request.Context())
	if resetError != nil {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"error":"Error while resetting database"}`))
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(200)
}
func main() {
	config := apiConfig{}
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		var response http.ResponseWriter
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{"error":"Something went wrong"}`))
		return
	}
	dbQueries := database.New(db)
	config.database = dbQueries
	mux := http.NewServeMux()
	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.Handle("/app/", config.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(200)
		response.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /admin/metrics", config.fetchMetrics)
	mux.HandleFunc("POST /admin/reset", config.resetMetrics)
	mux.HandleFunc("/api/login", func(response http.ResponseWriter, request *http.Request) {
		type requestJson struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		requestBody := requestJson{}
		responseBody := User{}
		if request.Method != http.MethodPost {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		decoder := json.NewDecoder(request.Body)
		decodeError := decoder.Decode(&requestBody)
		if decodeError != nil {
			response.WriteHeader(http.StatusBadRequest) // Should be 400 for bad request
			response.Write([]byte(`{"error":"issue with decoding json"}`))
			return
		}
		validateUser, validateEmailError := config.database.CheckUserWithEmail(request.Context(), requestBody.Email)
		if validateEmailError != nil {
			response.WriteHeader(http.StatusUnauthorized)
			response.Write([]byte(`{"error":"incorrect email or password"}`))
			return
		}
		validatePasswordError := auth.CheckPasswordHash(requestBody.Password, validateUser.HashedPassword)
		if validatePasswordError != nil {
			response.WriteHeader(http.StatusUnauthorized)
			response.Write([]byte(`{"error":"incorrect email or password"}`))
			return
		}
		responseBody.CreatedAt = validateUser.CreatedAt
		responseBody.UpdatedAt = validateUser.UpdatedAt
		responseBody.Email = validateUser.Email
		responseBody.ID = validateUser.ID
		responseJson, err := json.Marshal(responseBody)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"error while marshalling response json"}`))
			return
		}
		response.WriteHeader(http.StatusOK)
		response.Write(responseJson)

	})
	mux.HandleFunc("/api/users", func(response http.ResponseWriter, request *http.Request) {
		type requestJson struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		requestBody := requestJson{}
		responseBody := User{}
		if request.Method != http.MethodPost {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		decoder := json.NewDecoder(request.Body)
		decodeError := decoder.Decode(&requestBody)
		if decodeError != nil {
			response.WriteHeader(http.StatusBadRequest) // Should be 400 for bad request
			response.Write([]byte(`{"error":"issue with decoding json"}`))
			return
		}
		hashedPassword, hashError := auth.HashPassword(requestBody.Password)
		if hashError != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"Error while hashing password"}`))
			return
		}
		user, err := config.database.CreateUser(request.Context(), database.CreateUserParams{Email: requestBody.Email, HashedPassword: hashedPassword})
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"issue with creating user"}`))
			return
		}
		responseBody.CreatedAt = user.CreatedAt
		responseBody.Email = user.Email
		responseBody.ID = user.ID
		responseBody.UpdatedAt = user.UpdatedAt
		responseJsonData, err := json.Marshal(responseBody)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"issue with marshalling response json"}`))
			return
		}
		response.WriteHeader(http.StatusCreated)
		response.Write(responseJsonData)
	})
	mux.HandleFunc("/api/chirps", func(response http.ResponseWriter, request *http.Request) {
		type requestJson struct {
			Body   string `json:"body"`
			UserID string `json:"user_id"`
		}
		requestBody := requestJson{}
		responseBody := Chirp{}
		if request.Method != http.MethodPost {
			if request.Method == http.MethodGet {
				responseBody := []Chirp{}
				fetchedChirps, fetchError := config.database.GetChirpsByCreatedAt(request.Context())
				if fetchError != nil {
					response.WriteHeader(http.StatusInternalServerError)
					response.Write([]byte(`{"error":"error while fetching chirps"}`))
					return
				}
				for index := 0; index < len(fetchedChirps); index++ {
					responseBody = append(responseBody, Chirp{ID: fetchedChirps[index].ID, CreatedAt: fetchedChirps[index].CreatedAt, UpdatedAt: fetchedChirps[index].UpdatedAt, Body: fetchedChirps[index].Body, UserID: fetchedChirps[index].UserID})
				}
				responseJsonData, err := json.Marshal(responseBody)
				if err != nil {
					response.WriteHeader(http.StatusInternalServerError)
					response.Write([]byte(`{"error":"Something went wrong while marshalling chirps to json"}`))
					return
				}
				response.WriteHeader(http.StatusOK)
				response.Write(responseJsonData)
				return
			}
			response.WriteHeader(http.StatusMethodNotAllowed)
			return

		}
		restrictedWords := []string{"kerfuffle", "sharbert", "fornax"}
		response.Header().Set("Content-Type", "application/json")
		decoder := json.NewDecoder(request.Body)
		decodeError := decoder.Decode(&requestBody)
		if decodeError != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{"error":"Invalid request body"}`))
			return
		}
		charLimit := []rune(requestBody.Body)
		if len(charLimit) > 140 {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{"error":"Chirp is too long"}`))
			return
		}
		tempString := requestBody.Body
		tempStringParts := strings.Split(tempString, " ")
		for index, stringPart := range tempStringParts {
			for _, word := range restrictedWords {
				if word == strings.ToLower(stringPart) {
					tempStringParts[index] = "****"
				}
			}
		}
		joinedString := strings.Join(tempStringParts, " ")
		parsedUUID, uuidParseError := uuid.Parse(requestBody.UserID)
		if uuidParseError != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{"error":"Invalid user ID format"}`))
			return
		}
		_, userCheckError := config.database.CheckUser(request.Context(), parsedUUID)
		if userCheckError != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"User validation failed"}`))
			return
		}
		chirp, err := config.database.CreateChirp(request.Context(), database.CreateChirpParams{Body: joinedString, UserID: parsedUUID})
		// user, err := config.database.CreateUser(request.Context(), requestBody.Email)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"Chirp creation failed"}`))
			return
		}
		responseBody.CreatedAt = chirp.CreatedAt
		responseBody.Body = joinedString
		responseBody.ID = chirp.ID
		responseBody.UpdatedAt = chirp.UpdatedAt
		responseBody.UserID = chirp.UserID
		responseJsonData, err := json.Marshal(responseBody)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{"error":"Something went wrong"}`))
			return
		}
		response.WriteHeader(http.StatusCreated)
		response.Write(responseJsonData)
	})
	mux.HandleFunc("/api/chirps/{chirpID}", func(response http.ResponseWriter, request *http.Request) {
		responseBody := Chirp{}
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			response.Write([]byte(`{"error":"Something went wrong"}`))
			return
		}
		chirpID, uuidParseError := uuid.Parse(request.PathValue("chirpID"))
		if uuidParseError != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{"error":"Invalid user ID format"}`))
			return
		}
		fetchedChirp, fetchError := config.database.GetChirpByID(request.Context(), chirpID)
		if fetchError != nil {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(`{"error":"chirps not found"}`))
			return
		}
		responseBody.Body = fetchedChirp.Body
		responseBody.CreatedAt = fetchedChirp.CreatedAt
		responseBody.UpdatedAt = fetchedChirp.UpdatedAt
		responseBody.UserID = fetchedChirp.UserID
		responseBody.ID = fetchedChirp.ID
		responseJson, err := json.Marshal(responseBody)
		if err != nil {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(`{"error":"error while marshalling response json"}`))
			return
		}
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusOK)
		response.Write(responseJson)

	})
	err = server.ListenAndServe()
	log.Fatal(err)
}
