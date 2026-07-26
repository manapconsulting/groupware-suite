package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

var testRouter *mux.Router

func init() {
	// Initialize DB using the same connection as main app
	InitDB()

	// Setup router
	testRouter = mux.NewRouter()
	setupRoutes(testRouter)
}

func setupRoutes(r *mux.Router) {
	// Domains
	r.HandleFunc("/api/domains", GetDomains).Methods("GET")
	r.HandleFunc("/api/domains", CreateDomain).Methods("POST")
	r.HandleFunc("/api/domains/{id}", UpdateDomain).Methods("PUT")
	r.HandleFunc("/api/domains/{id}", DeleteDomain).Methods("DELETE")

	// Users
	r.HandleFunc("/api/users", GetUsers).Methods("GET")
	r.HandleFunc("/api/users", CreateUser).Methods("POST")
	r.HandleFunc("/api/users/{id}", UpdateUser).Methods("PUT")
	r.HandleFunc("/api/users/{id}", DeleteUser).Methods("DELETE")
	r.HandleFunc("/api/users/{id}/password", ChangePassword).Methods("PUT")

	// Aliases
	r.HandleFunc("/api/aliases", GetAliases).Methods("GET")
	r.HandleFunc("/api/aliases", CreateAlias).Methods("POST")
	r.HandleFunc("/api/aliases/{id}", DeleteAlias).Methods("DELETE")
}

// ============================================
// Domain Tests
// ============================================

func TestGetDomains(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/domains", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetDomains returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("expected success=true, got success=false")
	}
}

func TestDomainCRUD(t *testing.T) {
	testDomainName := "test-crud-domain.local"

	// CREATE
	t.Run("CreateDomain", func(t *testing.T) {
		domain := models.Domain{
			Name:           testDomainName,
			WebmailEnabled: true,
			CaldavEnabled:  true,
			CarddavEnabled: true,
		}

		body, _ := json.Marshal(domain)
		req, _ := http.NewRequest("POST", "/api/domains", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("CreateDomain returned wrong status code: got %v want %v, body: %s", status, http.StatusCreated, rr.Body.String())
		}
	})

	// Get domain ID for further tests
	domainID := getTestDomainID(t, testDomainName)

	// UPDATE
	t.Run("UpdateDomain", func(t *testing.T) {
		domain := models.Domain{
			Name:           testDomainName,
			WebmailEnabled: false,
			CaldavEnabled:  true,
			CarddavEnabled: false,
		}

		body, _ := json.Marshal(domain)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/domains/%d", domainID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("UpdateDomain returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// DELETE
	t.Run("DeleteDomain", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/domains/%d", domainID), nil)

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("DeleteDomain returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})
}

func TestCreateDomainInvalidJSON(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/domains", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestDeleteNonExistentDomain(t *testing.T) {
	req, _ := http.NewRequest("DELETE", "/api/domains/999999", nil)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	// Should return OK even for non-existent (idempotent delete)
	// or could return error - check actual behavior
	if status := rr.Code; status != http.StatusOK && status != http.StatusNotFound {
		t.Errorf("handler returned unexpected status code: got %v", status)
	}
}

// ============================================
// User Tests
// ============================================

func TestGetUsers(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/users", nil)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetUsers returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response models.APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("expected success=true, got success=false")
	}
}

func TestUserCRUD(t *testing.T) {
	testDomainName := "test-user-domain.local"
	testEmail := "testuser@test-user-domain.local"

	// Create domain first
	createTestDomainDirect(t, testDomainName)
	domainID := getTestDomainID(t, testDomainName)
	defer cleanupTestDomain(t, testDomainName)

	var userID int

	// CREATE USER
	t.Run("CreateUser", func(t *testing.T) {
		user := models.User{
			DomainID: domainID,
			Email:    testEmail,
			Password: "TestPassword123!",
		}

		body, _ := json.Marshal(user)
		req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("CreateUser returned wrong status code: got %v want %v, body: %s", status, http.StatusCreated, rr.Body.String())
			return
		}

		userID = getTestUserID(t, testEmail)
	})

	// UPDATE USER
	t.Run("UpdateUser", func(t *testing.T) {
		if userID == 0 {
			t.Skip("User not created")
		}

		user := models.User{
			DomainID: domainID,
			Email:    testEmail,
		}

		body, _ := json.Marshal(user)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/users/%d", userID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("UpdateUser returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// CHANGE PASSWORD
	t.Run("ChangePassword", func(t *testing.T) {
		if userID == 0 {
			t.Skip("User not created")
		}

		pc := models.PasswordChange{
			Password: "NewPassword456!",
		}

		body, _ := json.Marshal(pc)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/api/users/%d/password", userID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("ChangePassword returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})

	// DELETE USER
	t.Run("DeleteUser", func(t *testing.T) {
		if userID == 0 {
			t.Skip("User not created")
		}

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/users/%d", userID), nil)

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("DeleteUser returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})
}

func TestCreateUserWithoutPassword(t *testing.T) {
	testDomainName := "test-nopass-domain.local"
	createTestDomainDirect(t, testDomainName)
	domainID := getTestDomainID(t, testDomainName)
	defer cleanupTestDomain(t, testDomainName)

	user := models.User{
		DomainID: domainID,
		Email:    "nopass@test-nopass-domain.local",
	}

	body, _ := json.Marshal(user)
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("CreateUser without password should return 400: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestCreateUserInvalidJSON(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

// ============================================
// Alias Tests
// ============================================

func TestGetAliases(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/aliases", nil)

	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GetAliases returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestAliasCRUD(t *testing.T) {
	testDomainName := "test-alias-domain.local"
	testEmail := "aliasuser@test-alias-domain.local"
	testAliasSource := "testalias@test-alias-domain.local"

	// Create domain and user first
	createTestDomainDirect(t, testDomainName)
	domainID := getTestDomainID(t, testDomainName)
	createTestUserDirect(t, domainID, testEmail, "Password123")
	defer cleanupTestUser(t, testEmail)
	defer cleanupTestDomain(t, testDomainName)

	var aliasID int

	// CREATE ALIAS
	t.Run("CreateAlias", func(t *testing.T) {
		alias := models.Alias{
			DomainID:    domainID,
			Source:      testAliasSource,
			Destination: testEmail,
		}

		body, _ := json.Marshal(alias)
		req, _ := http.NewRequest("POST", "/api/aliases", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("CreateAlias returned wrong status code: got %v want %v, body: %s", status, http.StatusCreated, rr.Body.String())
			return
		}

		aliasID = getTestAliasID(t, testAliasSource)
	})

	// DELETE ALIAS
	t.Run("DeleteAlias", func(t *testing.T) {
		if aliasID == 0 {
			t.Skip("Alias not created")
		}

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/aliases/%d", aliasID), nil)

		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("DeleteAlias returned wrong status code: got %v want %v", status, http.StatusOK)
		}
	})
}

// ============================================
// Helper Functions
// ============================================

func createTestDomainDirect(t *testing.T, name string) {
	_, err := db.Exec("INSERT INTO virtual_domains (name, webmail_enabled, caldav_enabled, carddav_enabled) VALUES (?, 1, 1, 1)", name)
	if err != nil {
		t.Fatalf("failed to create test domain: %v", err)
	}
}

func getTestDomainID(t *testing.T, name string) int {
	var id int
	err := db.QueryRow("SELECT id FROM virtual_domains WHERE name = ?", name).Scan(&id)
	if err != nil {
		t.Fatalf("failed to get domain ID: %v", err)
	}
	return id
}

func cleanupTestDomain(t *testing.T, name string) {
	_, err := db.Exec("DELETE FROM virtual_domains WHERE name = ?", name)
	if err != nil {
		t.Logf("warning: failed to cleanup test domain: %v", err)
	}
}

func createTestUserDirect(t *testing.T, domainID int, email, password string) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash test password: %v", err)
	}
	_, err = db.Exec("INSERT INTO virtual_users (domain_id, email, password) VALUES (?, ?, ?)", domainID, email, hashedPassword)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
}

func getTestUserID(t *testing.T, email string) int {
	var id int
	err := db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&id)
	if err != nil {
		t.Fatalf("failed to get user ID: %v", err)
	}
	return id
}

func cleanupTestUser(t *testing.T, email string) {
	_, err := db.Exec("DELETE FROM virtual_users WHERE email = ?", email)
	if err != nil {
		t.Logf("warning: failed to cleanup test user: %v", err)
	}
}

func getTestAliasID(t *testing.T, source string) int {
	var id int
	err := db.QueryRow("SELECT id FROM virtual_aliases WHERE source = ?", source).Scan(&id)
	if err != nil {
		t.Fatalf("failed to get alias ID: %v", err)
	}
	return id
}

func cleanupTestAlias(t *testing.T, source string) {
	_, err := db.Exec("DELETE FROM virtual_aliases WHERE source = ?", source)
	if err != nil {
		t.Logf("warning: failed to cleanup test alias: %v", err)
	}
}

// ============================================
// Integration Test for API Response Format
// ============================================

func TestAPIResponseFormat(t *testing.T) {
	testCases := []struct {
		name     string
		method   string
		path     string
		wantCode int
	}{
		{"GetDomains", "GET", "/api/domains", http.StatusOK},
		{"GetUsers", "GET", "/api/users", http.StatusOK},
		{"GetAliases", "GET", "/api/aliases", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			testRouter.ServeHTTP(rr, req)

			if rr.Code != tc.wantCode {
				t.Errorf("%s returned wrong status: got %v want %v", tc.name, rr.Code, tc.wantCode)
			}

			var response models.APIResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Errorf("%s returned invalid JSON: %v", tc.name, err)
			}

			if !response.Success {
				t.Errorf("%s returned success=false", tc.name)
			}
		})
	}
}

// ============================================
// Benchmark Tests
// ============================================

func BenchmarkGetDomains(b *testing.B) {
	req, _ := http.NewRequest("GET", "/api/domains", nil)
	rr := httptest.NewRecorder()

	for i := 0; i < b.N; i++ {
		testRouter.ServeHTTP(rr, req)
	}
}

func BenchmarkGetUsers(b *testing.B) {
	req, _ := http.NewRequest("GET", "/api/users", nil)
	rr := httptest.NewRecorder()

	for i := 0; i < b.N; i++ {
		testRouter.ServeHTTP(rr, req)
	}
}

// Helper to convert int to string
func intToStr(i int) string {
	return strconv.Itoa(i)
}
