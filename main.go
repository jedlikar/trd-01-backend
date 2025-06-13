package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB
var apiKey string

type Signal struct {
	Data      string    `json:"data"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}
type HealthStatus struct {
	App string `json:"app"`
	DB  string `json:"db"`
}

// Middleware to check API key
func apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key != apiKey {
			log.Println("Wrong API key: ", key)
			log.Println("Current API key: ", apiKey)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	} else {
		ip = strings.Split(ip, ",")[0]
	}
	return ip
}

func postSignalHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Println("Invalid JSON: ", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ip := getClientIP(r)

	_, err := db.Exec(`INSERT INTO signal (data, ip_address) VALUES ($1, $2)`, payload.Data, ip)
	if err != nil {
		log.Println("Database error: ", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func getSignalHandler(w http.ResponseWriter, r *http.Request) {
	row := db.QueryRow(`SELECT data, ip_address, created_at FROM signal ORDER BY created_at DESC LIMIT 1`)

	var signal Signal
	err := row.Scan(&signal.Data, &signal.IPAddress, &signal.CreatedAt)
	if err == sql.ErrNoRows {
		log.Println("No signal found: ", err)
		http.Error(w, "No signal found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println("Database error: ", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signal)
}

func getHealthHandler(w http.ResponseWriter, r *http.Request) {

	status := HealthStatus{
		App: "ok",
		DB:  "ok",
	}

	if err := db.Ping(); err != nil {
		status.DB = "unreachable"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func main() {
	var err error
	err = godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file: ", err)
		log.Fatal("Error loading .env file")
	}

	apiKey = os.Getenv("API_KEY")
	port := os.Getenv("PORT")
	dbURL := os.Getenv("DATABASE_URL")

	log.Println("API Key: ", apiKey)
	log.Println("Running on port: ", port)
	log.Println("Using database URL: ", dbURL)
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Println("Database connection error: ", err)
		log.Fatal("Database connection error: ", err)
	}
	defer db.Close()

	r := mux.NewRouter()
	secure := r.PathPrefix("/").Subrouter()
	secure.Use(apiKeyMiddleware)
	secure.HandleFunc("/api/signal", postSignalHandler).Methods("POST")
	secure.HandleFunc("/api/signal", getSignalHandler).Methods("GET")
	secure.HandleFunc("/api/health", getHealthHandler).Methods("GET")

	fmt.Println("Server running on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
