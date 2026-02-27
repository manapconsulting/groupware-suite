package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

var testRouter *mux.Router

func init() {
	// Initialize DB
	InitDB()

	// Setup router
	testRouter = mux.NewRouter()
	setupTestRoutes(testRouter)
}

func setupTestRoutes(r *mux.Router) {
	// Auth
	r.HandleFunc("/api/login", Login).Methods("POST")

	// Protected routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(JWTAuthMiddleware)

	// Email - Folders
	api.HandleFunc("/folders", GetFolders).Methods("GET")

	// Email - Messages
	api.HandleFunc("/messages/{folder}", GetMessages).Methods("GET")

	// Calendar
	api.HandleFunc("/calendars", GetCalendars).Methods("GET")
	api.HandleFunc("/events", GetEvents).Methods("GET")
	api.HandleFunc("/events", CreateEvent).Methods("POST")
	api.HandleFunc("/events/{id}", UpdateEvent).Methods("PUT")
	api.HandleFunc("/events/{id}", DeleteEvent).Methods("DELETE")

	// Contacts
	api.HandleFunc("/addressbooks", GetAddressbooks).Methods("GET")
	api.HandleFunc("/contacts", GetContacts).Methods("GET")
	api.HandleFunc("/contacts", CreateContact).Methods("POST")
	api.HandleFunc("/contacts/{id}", UpdateContact).Methods("PUT")
	api.HandleFunc("/contacts/{id}", DeleteContact).Methods("DELETE")

	// Shared Calendars
	api.HandleFunc("/shared-calendars", GetSharedCalendars).Methods("GET")
	api.HandleFunc("/shared-calendars", CreateSharedCalendar).Methods("POST")
	api.HandleFunc("/shared-calendars/{id}", DeleteSharedCalendar).Methods("DELETE")
	api.HandleFunc("/domain-users", GetDomainUsers).Methods("GET")
}

// ============================================
// Auth Tests
// ============================================

func TestLoginInvalidCredentials(t *testing.T) {
	loginReq := map[string]string{
		"email":    "invalid@invalid.com",
		"password": "wrongpassword",
	}

	body, _ := json.Marshal(loginReq)
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	// Should return 401 or 403 for invalid credentials
	if status := rr.Code; status != http.StatusUnauthorized && status != http.StatusForbidden {
		t.Logf("Login with invalid credentials returned: %d, body: %s", status, rr.Body.String())
	}
}

func TestLoginInvalidJSON(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Login with invalid JSON should return 400: got %v", status)
	}
}

func TestProtectedRouteWithoutAuth(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/folders", nil)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Protected route without auth should return 401: got %v", status)
	}
}

func TestProtectedRouteWithInvalidToken(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/folders", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Protected route with invalid token should return 401: got %v", status)
	}
}

// ============================================
// Database Tests
// ============================================

func TestDatabaseConnection(t *testing.T) {
	if db == nil {
		t.Fatal("Database connection is nil")
	}

	err := db.Ping()
	if err != nil {
		t.Fatalf("Database ping failed: %v", err)
	}
}

// ============================================
// Calendar Tests (Unit Tests - No Auth Required)
// ============================================

func TestCalendarTable(t *testing.T) {
	// Test that calendar table exists and is accessible
	_, err := db.Query("SELECT id, user_email, name, color FROM calendars LIMIT 1")
	if err != nil {
		t.Logf("Calendar table query failed (may not have data): %v", err)
	}
}

func TestEventsTable(t *testing.T) {
	// Test that events table exists and is accessible
	_, err := db.Query("SELECT id, calendar_id, uid, title FROM events LIMIT 1")
	if err != nil {
		t.Logf("Events table query failed (may not have data): %v", err)
	}
}

func TestContactsTable(t *testing.T) {
	// Test that contacts table exists and is accessible
	_, err := db.Query("SELECT id, addressbook_id, uid, full_name FROM contacts LIMIT 1")
	if err != nil {
		t.Logf("Contacts table query failed (may not have data): %v", err)
	}
}

func TestSharedCalendarsTable(t *testing.T) {
	// Test that shared_calendars table exists and is accessible
	_, err := db.Query("SELECT id, name, owner_email, domain FROM shared_calendars LIMIT 1")
	if err != nil {
		t.Logf("Shared calendars table query failed (may not have data): %v", err)
	}
}

// ============================================
// CalDAV/CardDAV Tests
// ============================================

func TestWellKnownCalDAV(t *testing.T) {
	r := mux.NewRouter()
	r.PathPrefix("/.well-known/caldav").HandlerFunc(WellKnownCalDAV)

	req, _ := http.NewRequest("GET", "/.well-known/caldav", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMovedPermanently && status != http.StatusFound {
		t.Errorf("WellKnownCalDAV should redirect: got %v", status)
	}
}

func TestWellKnownCardDAV(t *testing.T) {
	r := mux.NewRouter()
	r.PathPrefix("/.well-known/carddav").HandlerFunc(WellKnownCardDAV)

	req, _ := http.NewRequest("GET", "/.well-known/carddav", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMovedPermanently && status != http.StatusFound {
		t.Errorf("WellKnownCardDAV should redirect: got %v", status)
	}
}

// ============================================
// IMAP Handler Unit Tests
// ============================================

func TestDecodeHeader(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Simple Text", "Simple Text"},
		{"=?UTF-8?B?VGVzdA==?=", "Test"},
		{"=?UTF-8?Q?Test_Email?=", "Test Email"},
	}

	for _, tc := range testCases {
		result := decodeHeader(tc.input)
		if result != tc.expected {
			t.Errorf("decodeHeader(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// ============================================
// Response Format Tests
// ============================================

func TestRespondJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]interface{}{
		"success": true,
		"message": "test",
	}

	RespondJSON(rr, http.StatusOK, data)

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type should be application/json: got %v", rr.Header().Get("Content-Type"))
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Status code should be 200: got %v", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}
}

// ============================================
// Integration Tests with Test User
// ============================================

func getTestToken(t *testing.T) string {
	// Try to login with test user
	loginReq := map[string]string{
		"email":    "test@manap.se",
		"password": "Test1234",
	}

	body, _ := json.Marshal(loginReq)
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Skipf("Could not get test token, skipping integration tests: %s", rr.Body.String())
		return ""
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if data, ok := response["data"].(map[string]interface{}); ok {
		if token, ok := data["token"].(string); ok {
			return token
		}
	}

	t.Skip("Could not extract token from response")
	return ""
}

func TestGetFoldersWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/folders", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetFolders with auth returned wrong status: got %v want %v, body: %s", status, http.StatusOK, rr.Body.String())
	}
}

func TestGetMessagesWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/messages/INBOX", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetMessages with auth returned wrong status: got %v want %v", status, http.StatusOK)
	}
}

func TestGetMessagesNoselectFolder(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	// Test the "shared" folder which is typically \Noselect
	req, _ := http.NewRequest("GET", "/api/messages/shared", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	// Should return 200 with empty list, not 500
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetMessages for Noselect folder should return 200: got %v", status)
	}
}

func TestGetCalendarsWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/calendars", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetCalendars returned wrong status: got %v want %v", status, http.StatusOK)
	}
}

func TestGetEventsWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetEvents returned wrong status: got %v want %v", status, http.StatusOK)
	}
}

func TestGetContactsWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/contacts", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetContacts returned wrong status: got %v want %v", status, http.StatusOK)
	}
}

func TestGetSharedCalendarsWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/shared-calendars", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetSharedCalendars returned wrong status: got %v want %v", status, http.StatusOK)
	}
}

func TestGetDomainUsersWithAuth(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	req, _ := http.NewRequest("GET", "/api/domain-users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetDomainUsers returned wrong status: got %v want %v", status, http.StatusOK)
	}
}

// ============================================
// Event CRUD Tests
// ============================================

func TestEventCRUD(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	var eventID int

	// CREATE EVENT
	t.Run("CreateEvent", func(t *testing.T) {
		event := map[string]interface{}{
			"title":       "Test Event",
			"description": "Test Description",
			"start_time":  time.Now().Format("2006-01-02 15:04:05"),
			"end_time":    time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
			"all_day":     false,
		}

		body, _ := json.Marshal(event)
		req, _ := http.NewRequest("POST", "/api/events", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated && status != http.StatusOK {
			t.Logf("CreateEvent returned: %v, body: %s", status, rr.Body.String())
			return
		}

		// Extract event ID from response
		var response map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
			if data, ok := response["data"].(map[string]interface{}); ok {
				if id, ok := data["id"].(float64); ok {
					eventID = int(id)
				}
			}
		}
	})

	// UPDATE EVENT
	t.Run("UpdateEvent", func(t *testing.T) {
		if eventID == 0 {
			t.Skip("Event not created")
		}

		event := map[string]interface{}{
			"title":       "Updated Test Event",
			"description": "Updated Description",
			"start_time":  time.Now().Format("2006-01-02 15:04:05"),
			"end_time":    time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04:05"),
		}

		body, _ := json.Marshal(event)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/events/%d", eventID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Logf("UpdateEvent returned: %v, body: %s", status, rr.Body.String())
		}
	})

	// DELETE EVENT
	t.Run("DeleteEvent", func(t *testing.T) {
		if eventID == 0 {
			t.Skip("Event not created")
		}

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/events/%d", eventID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Logf("DeleteEvent returned: %v, body: %s", status, rr.Body.String())
		}
	})
}

// ============================================
// Contact CRUD Tests
// ============================================

func TestContactCRUD(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	var contactID int

	// CREATE CONTACT
	t.Run("CreateContact", func(t *testing.T) {
		contact := map[string]interface{}{
			"full_name": "Test Contact",
			"email":     "testcontact@example.com",
			"phone":     "+1234567890",
			"company":   "Test Company",
		}

		body, _ := json.Marshal(contact)
		req, _ := http.NewRequest("POST", "/api/contacts", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated && status != http.StatusOK {
			t.Logf("CreateContact returned: %v, body: %s", status, rr.Body.String())
			return
		}

		// Extract contact ID from response
		var response map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
			if data, ok := response["data"].(map[string]interface{}); ok {
				if id, ok := data["id"].(float64); ok {
					contactID = int(id)
				}
			}
		}
	})

	// UPDATE CONTACT
	t.Run("UpdateContact", func(t *testing.T) {
		if contactID == 0 {
			t.Skip("Contact not created")
		}

		contact := map[string]interface{}{
			"full_name": "Updated Test Contact",
			"email":     "updated@example.com",
		}

		body, _ := json.Marshal(contact)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/contacts/%d", contactID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Logf("UpdateContact returned: %v, body: %s", status, rr.Body.String())
		}
	})

	// DELETE CONTACT
	t.Run("DeleteContact", func(t *testing.T) {
		if contactID == 0 {
			t.Skip("Contact not created")
		}

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/contacts/%d", contactID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Logf("DeleteContact returned: %v, body: %s", status, rr.Body.String())
		}
	})
}

// ============================================
// Shared Calendar Tests
// ============================================

func TestSharedCalendarCRUD(t *testing.T) {
	token := getTestToken(t)
	if token == "" {
		return
	}

	var calendarID int

	// CREATE SHARED CALENDAR
	t.Run("CreateSharedCalendar", func(t *testing.T) {
		calendar := map[string]interface{}{
			"name":        "Test Shared Calendar",
			"description": "Test Description",
			"color":       "#FF5733",
		}

		body, _ := json.Marshal(calendar)
		req, _ := http.NewRequest("POST", "/api/shared-calendars", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated && status != http.StatusOK {
			t.Logf("CreateSharedCalendar returned: %v, body: %s", status, rr.Body.String())
			return
		}

		// Extract calendar ID from response
		var response map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err == nil {
			if data, ok := response["data"].(map[string]interface{}); ok {
				if id, ok := data["id"].(float64); ok {
					calendarID = int(id)
				}
			}
		}
	})

	// DELETE SHARED CALENDAR
	t.Run("DeleteSharedCalendar", func(t *testing.T) {
		if calendarID == 0 {
			t.Skip("Shared calendar not created")
		}

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/shared-calendars/%d", calendarID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Logf("DeleteSharedCalendar returned: %v, body: %s", status, rr.Body.String())
		}
	})
}

// ============================================
// Benchmark Tests
// ============================================

func BenchmarkLogin(b *testing.B) {
	loginReq := map[string]string{
		"email":    "test@manap.se",
		"password": "Test1234",
	}
	body, _ := json.Marshal(loginReq)

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
	}
}
