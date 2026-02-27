package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Create groupware tables
	createTables()

	log.Println("Connected to database")
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS calendars (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			name VARCHAR(255) NOT NULL DEFAULT 'Calendar',
			color VARCHAR(7) DEFAULT '#3498db',
			description TEXT,
			ctag VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES virtual_users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id INT AUTO_INCREMENT PRIMARY KEY,
			calendar_id INT NOT NULL,
			uid VARCHAR(255) NOT NULL,
			summary VARCHAR(255),
			description TEXT,
			location VARCHAR(255),
			start_time DATETIME,
			end_time DATETIME,
			all_day BOOLEAN DEFAULT FALSE,
			rrule TEXT,
			ical_data TEXT,
			etag VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON DELETE CASCADE,
			UNIQUE KEY unique_uid (calendar_id, uid)
		)`,
		`CREATE TABLE IF NOT EXISTS addressbooks (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			name VARCHAR(255) NOT NULL DEFAULT 'Contacts',
			description TEXT,
			ctag VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES virtual_users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			id INT AUTO_INCREMENT PRIMARY KEY,
			addressbook_id INT NOT NULL,
			uid VARCHAR(255) NOT NULL,
			full_name VARCHAR(255),
			email VARCHAR(255),
			phone VARCHAR(50),
			vcard_data TEXT,
			etag VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (addressbook_id) REFERENCES addressbooks(id) ON DELETE CASCADE,
			UNIQUE KEY unique_uid (addressbook_id, uid)
		)`,
		`CREATE TABLE IF NOT EXISTS shared_calendars (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			color VARCHAR(7) DEFAULT '#9b59b6',
			description TEXT,
			owner_id INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES virtual_users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_members (
			id INT AUTO_INCREMENT PRIMARY KEY,
			calendar_id INT NOT NULL,
			user_id INT NOT NULL,
			role ENUM('viewer', 'editor') DEFAULT 'viewer',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (calendar_id) REFERENCES shared_calendars(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES virtual_users(id) ON DELETE CASCADE,
			UNIQUE KEY unique_member (calendar_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS shared_events (
			id INT AUTO_INCREMENT PRIMARY KEY,
			calendar_id INT NOT NULL,
			uid VARCHAR(255) NOT NULL,
			summary VARCHAR(255),
			description TEXT,
			location VARCHAR(255),
			start_time DATETIME,
			end_time DATETIME,
			all_day BOOLEAN DEFAULT FALSE,
			created_by INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (calendar_id) REFERENCES shared_calendars(id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES virtual_users(id) ON DELETE SET NULL,
			UNIQUE KEY unique_uid (calendar_id, uid)
		)`,
	}

	for _, q := range queries {
		_, err := db.Exec(q)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			log.Printf("Warning creating table: %v", err)
		}
	}
}

// BasicAuthMiddleware handles HTTP Basic Authentication for CalDAV/CardDAV only
func BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply basic auth to CalDAV/CardDAV paths
		// All other paths (/, /api/*, etc.) should pass through
		isCalDAV := strings.HasPrefix(r.URL.Path, "/caldav")
		isCardDAV := strings.HasPrefix(r.URL.Path, "/carddav")
		isWellKnown := strings.HasPrefix(r.URL.Path, "/.well-known/")

		// If not a CalDAV/CardDAV path, pass through without basic auth
		if !isCalDAV && !isCardDAV && !isWellKnown {
			next.ServeHTTP(w, r)
			return
		}

		// Well-known redirects don't need auth
		if isWellKnown {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Groupware"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(auth, "Basic ") {
			http.Error(w, "Invalid auth method", http.StatusUnauthorized)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(auth[6:])
		if err != nil {
			http.Error(w, "Invalid auth encoding", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			http.Error(w, "Invalid credentials format", http.StatusUnauthorized)
			return
		}

		email := parts[0]
		password := parts[1]

		// Check if service is enabled for this domain
		caldavEnabled, carddavEnabled, err := checkGroupwareEnabled(email)
		if err != nil {
			http.Error(w, "Domain not found", http.StatusForbidden)
			return
		}

		// Check based on path (isCalDAV and isCardDAV are already defined above)
		if isCalDAV && !caldavEnabled {
			http.Error(w, "CalDAV is disabled for this domain", http.StatusForbidden)
			return
		}
		if isCardDAV && !carddavEnabled {
			http.Error(w, "CardDAV is disabled for this domain", http.StatusForbidden)
			return
		}

		userID, err := authenticateUser(email, password)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Groupware"`)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Store user info in request context
		r.Header.Set("X-User-ID", string(rune(userID)))
		r.Header.Set("X-User-Email", email)

		// Ensure user has default calendar and addressbook
		ensureDefaultCollections(userID)

		next.ServeHTTP(w, r)
	})
}

func authenticateUser(email, password string) (int, error) {
	var userID int
	var storedHash string

	err := db.QueryRow("SELECT id, password FROM virtual_users WHERE email = ?", email).Scan(&userID, &storedHash)
	if err != nil {
		return 0, err
	}

	// Verify password using SHA512-CRYPT
	if verifyPassword(password, storedHash) {
		return userID, nil
	}

	return 0, sql.ErrNoRows
}

func verifyPassword(password, storedHash string) bool {
	// Use doveadm for password verification
	return verifyWithDoveadm(password, storedHash)
}

func verifyWithDoveadm(password, storedHash string) bool {
	// Use doveadm to verify password
	cmd := exec.Command("doveadm", "pw", "-t", storedHash, "-p", password)
	err := cmd.Run()
	return err == nil
}

func hashPassword(password string) (string, error) {
	cmd := exec.Command("doveadm", "pw", "-s", "SHA512-CRYPT", "-p", password)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func updateUserPassword(email, newPassword string) error {
	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE virtual_users SET password = ? WHERE email = ?", hashedPassword, email)
	return err
}

func ensureDefaultCollections(userID int) {
	// Create default calendar if not exists
	var calCount int
	db.QueryRow("SELECT COUNT(*) FROM calendars WHERE user_id = ?", userID).Scan(&calCount)
	if calCount == 0 {
		db.Exec("INSERT INTO calendars (user_id, name, ctag) VALUES (?, 'Calendar', ?)", userID, generateETag())
	}

	// Create default addressbook if not exists
	var abCount int
	db.QueryRow("SELECT COUNT(*) FROM addressbooks WHERE user_id = ?", userID).Scan(&abCount)
	if abCount == 0 {
		db.Exec("INSERT INTO addressbooks (user_id, name, ctag) VALUES (?, 'Contacts', ?)", userID, generateETag())
	}
}

// checkGroupwareEnabled checks if CalDAV/CardDAV is enabled for the given email's domain
func checkGroupwareEnabled(email string) (caldavEnabled bool, carddavEnabled bool, err error) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false, false, fmt.Errorf("invalid email format")
	}
	domain := parts[1]

	// Call mailadmin API to check if caldav/carddav is enabled
	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/api/groupware-check/%s", domain))
	if err != nil {
		// If mailadmin is unreachable, allow access (fail open)
		return true, true, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, false, fmt.Errorf("domain not found")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, true, nil
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			CaldavEnabled  bool `json:"caldav_enabled"`
			CarddavEnabled bool `json:"carddav_enabled"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return true, true, nil
	}

	return result.Data.CaldavEnabled, result.Data.CarddavEnabled, nil
}
