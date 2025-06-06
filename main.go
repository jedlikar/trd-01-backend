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
	_ "github.com/lib/pq"
)

var db *sql.DB

const apiKey = "your-secret-api-key" // TODO: Change this to something secret

type Signal struct {
	Data      string    `json:"data"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

// Middleware to check API key
func apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key != apiKey {
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
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ip := getClientIP(r)

	_, err := db.Exec(`INSERT INTO signal (data, ip_address) VALUES ($1, $2)`, payload.Data, ip)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func getSignalHandler(w http.ResponseWriter, r *http.Request) {
	row := db.QueryRow(`SELECT data, ip_address, created_at FROM Signal ORDER BY created_at DESC LIMIT 1`)

	var signal Signal
	err := row.Scan(&signal.Data, &signal.IPAddress, &signal.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "No signal found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(signal)
}

func main() {
	var err error
	db, err = sql.Open("postgres", "postgres://trd_user:strongpassword123@db:5432/trd_db?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := mux.NewRouter()
	secure := r.PathPrefix("/").Subrouter()
	secure.Use(apiKeyMiddleware)
	secure.HandleFunc("/signal", postSignalHandler).Methods("POST")
	secure.HandleFunc("/signal", getSignalHandler).Methods("GET")

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	fmt.Println("Server running on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
