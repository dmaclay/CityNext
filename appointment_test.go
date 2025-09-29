package main

/* Created tests via Copilot prompt:

Write go http test calls for service
Check out of year dates, earlier in year dates, duplicate day appointments, public holiday clashes, invalid date, missing first name, missing last name

Public holidays  for test year:
Loading public holidays for 2075 in GB...
Loaded holiday: 2075-01-01 - New Year's Day
Loaded holiday: 2075-01-02 - 2 January
Loaded holiday: 2075-03-18 - Saint Patrick's Day
Loaded holiday: 2075-04-05 - Good Friday
Loaded holiday: 2075-04-08 - Easter Monday
Loaded holiday: 2075-05-06 - Early May Bank Holiday
Loaded holiday: 2075-05-27 - Spring Bank Holiday
Loaded holiday: 2075-07-12 - Battle of the Boyne
Loaded holiday: 2075-08-05 - Summer Bank Holiday
Loaded holiday: 2075-08-26 - Summer Bank Holiday
Loaded holiday: 2075-12-02 - Saint Andrew's Day
Loaded holiday: 2075-12-25 - Christmas Day
Loaded holiday: 2075-12-26 - Boxing Day
Successfully loaded 13 public holidays for 2075

//*/

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestServer(t *testing.T) *Server {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}

	server := NewServer(db)

	if err := server.initDB(); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Load test holidays manually
	server.publicHolidays = map[string]bool{
		"2075-01-01": true,
		"2075-01-02": true,
		"2075-03-18": true,
		"2075-04-05": true,
		"2075-04-08": true,
		"2075-05-06": true,
		"2075-05-27": true,
		"2075-07-12": true,
		"2075-08-05": true,
		"2075-08-26": true,
		"2075-12-02": true,
		"2075-12-25": true,
		"2075-12-26": true,
	}

	server.yearStr = "2075"

	// Override "today" for testing
	fakeToday, _ := time.Parse("2006-01-02", "2075-01-01")
	server.todayOverride = &fakeToday

	return server
}

func postAppointment(t *testing.T, handler http.Handler, req AppointmentRequest) *httptest.ResponseRecorder {
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/appointments", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestOutOfYearDate(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "Alice",
		LastName:  "OutOfYear",
		VisitDate: "2074-06-15",
	})

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for out-of-year, got %d", resp.Code)
	}
}

func TestEarlyYearDate(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "Bob",
		LastName:  "TooEarly",
		VisitDate: "2075-01-01",
	})

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for early-year date, got %d", resp.Code)
	}
}

func TestPublicHolidayClash(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "Charlie",
		LastName:  "HolidayClash",
		VisitDate: "2075-12-25",
	})

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for public holiday clash, got %d", resp.Code)
	}
}

func TestValidAppointment(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "Dana",
		LastName:  "Valid",
		VisitDate: "2075-06-15",
	})

	if resp.Code != http.StatusCreated {
		t.Errorf("Expected 201 for valid appointment, got %d", resp.Code)
	}
}

func TestDuplicateAppointment(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	req := AppointmentRequest{
		FirstName: "Dana",
		LastName:  "Valid",
		VisitDate: "2075-06-15",
	}

	// First attempt
	resp1 := postAppointment(t, router, req)
	if resp1.Code != http.StatusCreated {
		t.Errorf("Expected 201 for first appointment, got %d", resp1.Code)
	}

	// Second attempt (duplicate)
	resp2 := postAppointment(t, router, req)
	if resp2.Code != http.StatusConflict {
		t.Errorf("Expected 409 for duplicate appointment, got %d", resp2.Code)
	}
}

func TestInvalidDateFormat(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	// Bad date format (slashes instead of dashes)
	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "Eve",
		LastName:  "BadDate",
		VisitDate: "2075/06/15",
	})

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid date format, got %d", resp.Code)
	}
}

func TestMissingFirstName(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "",
		LastName:  "NoFirst",
		VisitDate: "2075-06-15",
	})

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing first name, got %d", resp.Code)
	}
}

func TestMissingLastName(t *testing.T) {
	server := setupTestServer(t)
	router := mux.NewRouter()
	router.HandleFunc("/appointments", server.createAppointment).Methods("POST")

	resp := postAppointment(t, router, AppointmentRequest{
		FirstName: "NoLast",
		LastName:  "",
		VisitDate: "2075-06-15",
	})

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing last name, got %d", resp.Code)
	}
}
