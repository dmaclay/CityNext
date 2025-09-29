package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// For convienience
func init() {
	log.SetOutput(os.Stdout)
}

// We need public holidays from the Nager date API
type PublicHoliday struct {
	Date        string   `json:"date"`
	LocalName   string   `json:"localName"`
	Name        string   `json:"name"`
	CountryCode string   `json:"countryCode"`
	Fixed       bool     `json:"fixed"`
	Global      bool     `json:"global"`
	Counties    []string `json:"counties"`
	LaunchYear  int      `json:"launchYear"`
	Types       []string `json:"types"`
}

// Since the Nager data used camelCase ... stick with that

// Now we need the appointment on the db
type Appointment struct {
	ID        int       `json:"id"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	VisitDate string    `json:"visitDate"`
	CreatedAt time.Time `json:"createdAt"`
}

// And we need the appointment request that might no make it onto the db
type AppointmentRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	VisitDate string `json:"visitDate"`
}

// Errors
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Since it is 2075 and thus a single year we should have the server
// fetch all the public holidays for the year on start.
// Still, lets not hardcode the year, rather pass in on on start
// The country (GB) we will hardcode
// So we just need a server with a db of appointments, and a map of public holidays
type Server struct {
	db             *sql.DB
	publicHolidays map[string]bool
	yearStr        string
	todayOverride  *time.Time // just for testing
}

func NewServer(db *sql.DB) *Server {
	return &Server{
		db:             db,
		publicHolidays: make(map[string]bool),
	}
}

// Setup table for above appoiuntment
func (s *Server) initDB() error {
	query := `
	CREATE TABLE IF NOT EXISTS appointments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		visit_date TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`

	_, err := s.db.Exec(query)
	return err
}

// Send error ... there's gonna be a lot of options
func (s *Server) sendErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errorType,
		Message: message,
	})
}

// Load UK public holidays for 2075 or whatever year we pick into memory
func (s *Server) loadPublicHolidays(yearStr string, countryCode string) error {
	url := fmt.Sprintf("https://date.nager.at/api/v3/PublicHolidays/%s/%s", yearStr, countryCode)
	log.Printf("Loading public holidays for %s in %s...", yearStr, countryCode)

	// Remember the year for future appointment validation
	s.yearStr = yearStr

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch public holidays: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("public holiday API returned status: %d", resp.StatusCode)
	}

	var holidays []PublicHoliday
	if err := json.NewDecoder(resp.Body).Decode(&holidays); err != nil {
		return fmt.Errorf("failed to decode public holidays: %w", err)
	}

	// Cache public holidays in map
	for _, holiday := range holidays {
		s.publicHolidays[holiday.Date] = true
		log.Printf("Loaded holiday: %s - %s", holiday.Date, holiday.LocalName)
	}

	log.Printf("Successfully loaded %d public holidays for 2075", len(holidays))
	return nil
}

// Check if a new date is one of the public holidays
func (s *Server) isPublicHoliday(visitDate time.Time) bool {
	visitDateStr := visitDate.Format("2006-01-02")
	return s.publicHolidays[visitDateStr]
}

// Check if a new date is already exists on db as an appointment
func (s *Server) appointmentExists(visitDate time.Time) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM appointments WHERE visit_date = ?"
	err := s.db.QueryRow(query, visitDate.Format("2006-01-02")).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// The appointment handler,
// really most of the conditional checks and validation,
// which only gets called if you are trying to create a new appointment
// although that's all you can do
// A 'real' system would always have a page/endpoint to list all current appointments etc.
func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {

	// Construct a fake "today" using Now() and the server year
	year, err := strconv.Atoi(s.yearStr)
	if err != nil {
		fmt.Printf("Invalid year: %v\n", err)
		return
	}

	var today time.Time
	if s.todayOverride != nil { // Just for testing
		today = *s.todayOverride
	} else {
		now := time.Now().UTC()
		today = time.Date(year, now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}
	// fmt.Printf("Constructed date: %s\n", today.Format("2006-01-02"))

	if r.Method != http.MethodPost {
		s.sendErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST method is allowed")
		return
	}

	var req AppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON format")
		return
	}

	// Validate required fields
	if req.FirstName == "" || req.LastName == "" || req.VisitDate == "" {
		s.sendErrorResponse(w, http.StatusBadRequest, "missing_fields", "First name, last name, and visit date are required")
		return
	}

	// Parse and validate visit date
	visitDate, err := time.Parse("2006-01-02", req.VisitDate)
	if err != nil {
		s.sendErrorResponse(w, http.StatusBadRequest, "invalid_date", "Visit date must be in YYYY-MM-DD format")
		return
	}

	// Validate year is 2075
	if visitDate.Year() != today.Year() {
		s.sendErrorResponse(w, http.StatusBadRequest, "invalid_year", "Appointments can only be scheduled for year 2075")
		return
	}

	// Check if date is earlier this year
	if visitDate.Before(today) {
		s.sendErrorResponse(w, http.StatusBadRequest, "past_date", "Visit date cannot be in the past")
		return
	}

	// Check if date is a public holiday
	if s.isPublicHoliday(visitDate) {
		s.sendErrorResponse(w, http.StatusBadRequest, "public_holiday", "Appointments cannot be scheduled on public holidays")
		return
	}

	// Check for duplicate appointment
	exists, err := s.appointmentExists(visitDate)
	if err != nil {
		log.Printf("Error checking existing appointments: %v", err)
		s.sendErrorResponse(w, http.StatusInternalServerError, "database_error", "Failed checking existing appointments")
		return
	}

	if exists {
		s.sendErrorResponse(w, http.StatusConflict, "duplicate_appointment", "An appointment is already Scheduled for this date")
		return
	}

	// Create the appointment
	var appointment Appointment
	query := `
		INSERT INTO appointments (first_name, last_name, visit_date) 
		VALUES (?, ?, ?) 
		RETURNING id, first_name, last_name, visit_date, created_at`

	err = s.db.QueryRow(query, req.FirstName, req.LastName, visitDate.Format("2006-01-02")).Scan(
		&appointment.ID,
		&appointment.FirstName,
		&appointment.LastName,
		&appointment.VisitDate,
		&appointment.CreatedAt,
	)

	if err != nil {
		log.Printf("Error creating appointment: %v", err)
		s.sendErrorResponse(w, http.StatusInternalServerError, "database_error", "Failed to create appointment")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(appointment)
}
func main() {
	//Santiy check
	fmt.Println("Starting server...")

	// Set defaults for country and year
	// We assume country is always GB, but lets keep it near the yearStr
	countryCode := "GB"
	yearStr := "2075"

	// Take the year from the commandline and build a fake "now" date
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <year>")
		return
	}

	yearStr = os.Args[1]

	dbPath := "./appointments.db"
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?cache=shared&mode=rwc")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check it's live
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Printf("Connected to SQLite database: %s\n", dbPath)

	server := NewServer(db)

	// fmt.Printf("%+v\n", server)

	// Now we need those public holidays
	if err := server.loadPublicHolidays(yearStr, countryCode); err != nil {
		log.Fatal("Failed to load public holidays:", err)
	}

	// fmt.Printf("%+v\n", server)

	// Initialise our db table
	if err := server.initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// The routing ... to /appointments ... our only endpoint and just for POST
	r := mux.NewRouter()
	r.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	port := ":8080"
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, r))

}
