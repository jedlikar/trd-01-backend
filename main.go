package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
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
func postSignalFileHandler(w http.ResponseWriter, r *http.Request) {
	//if r.Method != http.MethodPost {
	//	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	//	return
	//}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println("Invalid file upload: ", err)
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ip := getClientIP(r)

	tx, err := db.Begin()
	if err != nil {
		log.Println("DB error: ", err)
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Insert upload record
	var sourceFileId int
	err = tx.QueryRow(`
        INSERT INTO source_file (filename, source_ip_address) 
        VALUES ($1, $2) RETURNING id
    `, header.Filename, ip).Scan(&sourceFileId)
	if err != nil {
		log.Println("Failed to insert upload info: ", err)
		http.Error(w, "Failed to insert upload info", http.StatusInternalServerError)
		return
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	_, err = reader.Read() // skip header
	if err != nil {
		log.Println("Invalid CSV format: ", err)
		http.Error(w, "Invalid CSV format", http.StatusBadRequest)
		return
	}

	stmt, err := tx.Prepare(`
        INSERT INTO market_signal (
            symbol, action, quantity, sectype, exchange,
            time_in_force, order_type, lmt_price, order_id,
            basket_tag, order_ref, account, aux_price, parent_order_id, source_file_id
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
    `)
	if err != nil {
		log.Println("Failed to prepare statement: ", err)
		http.Error(w, "Failed to prepare statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("CSV read error: ", err)
			http.Error(w, "CSV read error", http.StatusBadRequest)
			return
		}

		quantity, _ := strconv.ParseFloat(record[2], 64)
		lmtPrice, _ := strconv.ParseFloat(record[7], 64)
		orderID, _ := strconv.Atoi(record[8])
		auxPrice, _ := strconv.ParseFloat(record[12], 64)
		parentOrderID, _ := strconv.Atoi(record[13])

		_, err = stmt.Exec(
			record[0], record[1], quantity, record[3], record[4],
			record[5], record[6], lmtPrice, orderID,
			record[9], record[10], record[11], auxPrice, parentOrderID, sourceFileId,
		)
		if err != nil {
			log.Println("DB insert failed: ", err)
			http.Error(w, "DB insert failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Println("Transaction commit failed: ", err)
		http.Error(w, "Transaction commit failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "CSV uploaded successfully (source_file_id = %d)\n", sourceFileId)
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
	secure.HandleFunc("/api/signal_file", postSignalFileHandler).Methods("POST")

	fmt.Println("Server running on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
