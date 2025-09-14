package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// PaymentRequest defines the structure for incoming payment requests
type PaymentRequest struct {
	CardNumber string  `json:"card_number"`
	Expiry     string  `json:"expiry"`
	CVV        string  `json:"cvv"`
	Amount     float64 `json:"amount"`
}

// PaymentResponse defines the structure for payment responses
type PaymentResponse struct {
	Message       string `json:"message"`
	TransactionID int    `json:"transaction_id"`
}

// Transaction defines the structure for stored transactions
type Transaction struct {
	ID        int
	Token     string
	Amount    float64
	Status    string
	CreatedAt time.Time
}

var db *sql.DB

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Initialize database connection
	connStr := "user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" host=" + os.Getenv("DB_HOST") +
		" port=" + os.Getenv("DB_PORT") +
		" sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	defer db.Close()

	// Test database connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Database ping failed: ", err)
	}

	// Serve static files (HTML, CSS, JS)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// API endpoint for payment processing
	http.HandleFunc("/api/payments", handlePayment)

	// Start server
	log.Println("Server starting on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// handlePayment processes incoming payment requests
func handlePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PaymentRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Input validation
	if !validateCardNumber(req.CardNumber) {
		http.Error(w, "Invalid card number", http.StatusBadRequest)
		return
	}
	if !validateExpiry(req.Expiry) {
		http.Error(w, "Invalid expiry date", http.StatusBadRequest)
		return
	}
	if !validateCVV(req.CVV) {
		http.Error(w, "Invalid CVV", http.StatusBadRequest)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	// Tokenize card details
	token := tokenizeCard(req.CardNumber)

	// Process payment and store transaction
	transactionID, success := processAndStorePayment(token, req.Amount, req.Expiry, req.CVV)

	// Log transaction
	log.Printf("Payment processed: token=%s, amount=%.2f, success=%v, transaction_id=%d, time=%v",
		token, req.Amount, success, transactionID, time.Now())

	w.Header().Set("Content-Type", "application/json")
	resp := PaymentResponse{TransactionID: transactionID}
	if success {
		resp.Message = "Payment successful"
		w.WriteHeader(http.StatusOK)
	} else {
		resp.Message = "Payment failed"
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

// validateCardNumber checks if the card number is valid using the Luhn algorithm
func validateCardNumber(cardNumber string) bool {
	cardNumber = regexp.MustCompile(`\s+`).ReplaceAllString(cardNumber, "")
	if len(cardNumber) != 16 {
		return false
	}
	sum := 0
	isEven := false
	for i := len(cardNumber) - 1; i >= 0; i-- {
		digit, _ := strconv.Atoi(string(cardNumber[i]))
		if isEven {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		isEven = !isEven
	}
	return sum%10 == 0
}

// validateExpiry checks if the expiry date is valid and not in the past
func validateExpiry(expiry string) bool {
	matched, _ := regexp.MatchString(`^\d{2}/\d{2}$`, expiry)
	if !matched {
		return false
	}
	parts := regexp.MustCompile(`/`).Split(expiry, -1)
	month, _ := strconv.Atoi(parts[0])
	year, _ := strconv.Atoi(parts[1])
	currentYear := time.Now().Year() % 100
	currentMonth := int(time.Now().Month())
	return month >= 1 && month <= 12 && year >= currentYear && (year > currentYear || month >= currentMonth)
}

// validateCVV checks if the CVV is a 3-digit number
func validateCVV(cvv string) bool {
	matched, _ := regexp.MatchString(`^\d{3}$`, cvv)
	return matched
}

// tokenizeCard generates a secure token from the card number
func tokenizeCard(cardNumber string) string {
	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		log.Fatal("SECRET_KEY environment variable not set")
	}
	hash := sha256.Sum256([]byte(cardNumber + secretKey))
	return hex.EncodeToString(hash[:])
}

// processAndStorePayment processes the payment and stores it in the database
func processAndStorePayment(token string, amount float64, expiry, cvv string) (int, bool) {
	// Simulate payment processor interaction
	success := processPayment(token, amount, expiry, cvv)

	// Store transaction
	status := "failed"
	if success {
		status = "success"
	}
	var transactionID int
	err := db.QueryRow(
		"INSERT INTO transactions (token, amount, status, created_at) VALUES ($1, $2, $3, $4) RETURNING id",
		token, amount, status, time.Now(),
	).Scan(&transactionID)
	if err != nil {
		log.Printf("Failed to store transaction: %v", err)
		return 0, false
	}

	return transactionID, success
}

// processPayment simulates interaction with a payment processor
func processPayment(token string, amount float64, expiry, cvv string) bool {
	if token == "" {
		log.Printf("Payment failed: empty token")
		return false
	}
	if amount <= 0 {
		log.Printf("Payment failed: invalid amount %.2f", amount)
		return false
	}
	if expiry == "" {
		log.Printf("Payment failed: empty expiry")
		return false
	}
	if cvv == "" {
		log.Printf("Payment failed: empty CVV")
		return false
	}
	if len(cvv) != 3 {
		log.Printf("Payment failed: invalid CVV length")
		return false
	}
	// Simulate payment processor interaction 
	// Random failure simulation (20% chance)
	/*	if time.Now().UnixNano()%10 < 2 {
		log.Printf("Payment failed: processor declined (token=%s, amount=%.2f)", token, amount)
		return false
	}*/
	log.Printf("Payment approved: token=%s, amount=%.2f", token, amount)
	return true
}
