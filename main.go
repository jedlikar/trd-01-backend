package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	//"strconv"
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

// SaveUploadedFile stores the uploaded file in a dated folder and handles name collisions.
func SaveUploadedFile(fileBytes []byte, originalFilename string) (string, error) {
	// Create directory with today's date
	dateFolder := time.Now().Format("2006-01-02")
	baseDir := "/var/lib/trd-01/uploads"
	fullDir := filepath.Join(baseDir, dateFolder)

	err := os.MkdirAll(fullDir, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Handle filename collisions
	filename := originalFilename
	targetPath := filepath.Join(fullDir, filename)
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	counter := 1

	for {
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			break // File doesn't exist, we can use this name
		}
		// File exists, try with increment
		filename = fmt.Sprintf("%s_%d%s", name, counter, ext)
		targetPath = filepath.Join(fullDir, filename)
		counter++
	}

	// Save the file
	outFile, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, bytes.NewReader(fileBytes))
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return targetPath, nil
}

func indexOf(slice []string, target string) int {
	for i, s := range slice {
		if strings.ToLower(strings.TrimSpace(s)) == target {
			return i
		}
	}
	return -1
}

func postSignalFileHandler(w http.ResponseWriter, r *http.Request) {

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println("Invalid file upload: ", err)
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read the entire file into memory
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	// Save file to disk
	savedPath, err := SaveUploadedFile(fileBytes, header.Filename)
	if err != nil {
		log.Println("Failed to save uploaded file: ", err)
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File saved as: %s\n", savedPath)

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
        INSERT INTO source_file (name, path, source_ip_address) 
        VALUES ($1, $2, $3) RETURNING id
    `, header.Filename, savedPath, ip).Scan(&sourceFileId)
	if err != nil {
		log.Println("Failed to insert upload info: ", err)
		http.Error(w, "Failed to insert upload info", http.StatusInternalServerError)
		return
	}

	//tx.Commit()

	reader := csv.NewReader(bytes.NewReader(fileBytes))
	reader.TrimLeadingSpace = true
	csvHeader, err := reader.Read()
	if err != nil {
		log.Println("Failed to read header: ", err)
		http.Error(w, "Failed to read header", http.StatusBadRequest)
		return
	}

	// Prepare list of columns that exist in the DB
	columnMapping := map[string]string{
		"symbol":        "symbol",
		"action":        "action",
		"quantity":      "quantity",
		"sectype":       "sectype",
		"exchange":      "exchange",
		"timeinforce":   "time_in_force",
		"ordertype":     "order_type",
		"lmtprice":      "lmt_price",
		"orderid":       "order_id",
		"baskettag":     "basket_tag",
		"orderref":      "order_ref",
		"account":       "account",
		"auxprice":      "aux_price",
		"parentorderid": "parent_order_id",
	}

	// Filter valid columns from header
	var cols []string
	for _, col := range csvHeader {
		if mapped, ok := columnMapping[strings.ToLower(strings.TrimSpace(col))]; ok {
			cols = append(cols, mapped)
		} else {
			log.Printf("Unknown CSV column: %s\n", col)
			http.Error(w, "Unknown CSV column:", http.StatusBadRequest)
			return
		}
	}

	if len(cols) == 0 {
		log.Println("No valid columns found")
		http.Error(w, "No valid columns found", http.StatusBadRequest)
		return
	}

	// Build query dynamically
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf("INSERT INTO market_signal (%s, source_file_id) VALUES (%s, $%d)",
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		len(cols)+1,
	)
	// Read data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Invalid CSV format: ", err)
			http.Error(w, "Invalid CSV format", http.StatusBadRequest)
			return
		}

		// Build values slice based on selected columns
		values := make([]interface{}, len(cols))
		log.Println("cols: ", cols)
		for i, _ := range cols {
			//idx := indexOf(csvHeader, col)
			//if idx >= 0 && idx < len(record) {
			log.Println("cols[i]: ", cols[i])
			log.Println("record[i]: ", record[i])
			if record[i] != "" {
				values[i] = record[i]
			} else {
				values[i] = nil
			}
		}
		values = append(values, sourceFileId)

		_, err = tx.Exec(query, values...)
		if err != nil {
			log.Println("Insert failed: ", err)
			http.Error(w, "Insert failed", http.StatusBadRequest)
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
