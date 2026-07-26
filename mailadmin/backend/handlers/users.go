package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"mailadmin/models"

	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

// mysqlErrDupEntry is the server error code for a unique key violation.
const mysqlErrDupEntry = 1062

// Local part kept to a path-safe subset: it becomes a directory name under
// /var/mail/vhosts/<domain>/.
var localPartRE = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+$`)

var domainRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

// splitAddress validates an email address and returns its local and domain parts.
func splitAddress(email string) (string, string, error) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", "", errors.New("email must be in the form user@domain")
	}
	local, domain := parts[0], parts[1]

	if len(email) > 100 {
		return "", "", errors.New("email must be at most 100 characters")
	}
	if !localPartRE.MatchString(local) {
		return "", "", errors.New("local part may only contain letters, digits and . _ % + -")
	}
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") {
		return "", "", errors.New("local part may not start, end or contain consecutive dots")
	}
	if !domainRE.MatchString(domain) {
		return "", "", errors.New("invalid domain part")
	}
	return local, domain, nil
}

func GetUsers(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT u.id, u.domain_id, u.email, COALESCE(u.quota, 0), d.name as domain
		FROM virtual_users u
		JOIN virtual_domains d ON u.domain_id = d.id
		ORDER BY u.email
	`
	rows, err := db.Query(query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.DomainID, &u.Email, &u.Quota, &u.Domain); err != nil {
			continue
		}
		users = append(users, u)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: users})
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	_, emailDomain, err := splitAddress(user.Email)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	if user.Password == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Password required"})
		return
	}

	if user.Quota < 0 {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Quota must be 0 (unlimited) or a positive number of megabytes"})
		return
	}

	// The address must belong to the domain it is filed under, otherwise mail
	// for it would never be routed here.
	var domainName string
	err = db.QueryRow("SELECT name FROM virtual_domains WHERE id = ?", user.DomainID).Scan(&domainName)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Unknown domain_id"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	if !strings.EqualFold(domainName, emailDomain) {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{
			Success: false,
			Message: fmt.Sprintf("Email domain %q does not match domain %q", emailDomain, domainName),
		})
		return
	}

	// Keep plain password for the welcome email before it is hashed away.
	plainPassword := user.Password

	id, status, err := insertMailbox(&user)
	if err != nil {
		respondJSON(w, status, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	user.ID = id

	// Send welcome email if notify_email is provided
	if user.NotifyEmail != "" {
		// Capture values before goroutine (avoid race condition with clearing below)
		notifyEmail := user.NotifyEmail
		domain := ExtractDomain(user.Email)
		emailData := WelcomeEmailData{
			Email:    user.Email,
			Password: plainPassword,
			Domain:   domain,
		}
		// Send email in background (don't block response)
		go func() {
			if err := SendWelcomeEmail(notifyEmail, emailData); err != nil {
				// Log error but don't fail the user creation
				println("Failed to send welcome email:", err.Error())
			}
		}()
	}

	user.Password = ""
	user.NotifyEmail = ""

	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: user})
}

// insertMailbox performs the shared work of creating a mailbox: it inserts the
// virtual_users row, provisions the on-disk Maildir (unwinding the row if that
// fails), and seeds the groupware calendar/addressbook. Callers must have
// already validated the address and confirmed user.DomainID matches its domain.
// On failure it returns an HTTP status to relay together with the error.
func insertMailbox(user *models.User) (int, int, error) {
	hashedPassword, err := hashPassword(user.Password)
	if err != nil {
		return 0, http.StatusInternalServerError, errors.New("Failed to hash password: " + err.Error())
	}

	result, err := db.Exec(
		"INSERT INTO virtual_users (domain_id, email, password, quota) VALUES (?, ?, ?, ?)",
		user.DomainID, user.Email, hashedPassword, user.Quota,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrDupEntry {
			return 0, http.StatusConflict, errors.New("Email address already exists")
		}
		return 0, http.StatusInternalServerError, err
	}

	id64, _ := result.LastInsertId()
	id := int(id64)

	// Create the mailbox on disk so delivery works before the first login.
	if err := CreateMaildir(user.Email); err != nil {
		// Unwind: an account without a mailbox would bounce mail silently.
		db.Exec("DELETE FROM virtual_users WHERE id = ?", id)
		if rmErr := RemoveMaildir(user.Email); rmErr != nil {
			println("Failed to clean up maildir:", rmErr.Error())
		}
		return 0, http.StatusInternalServerError, errors.New("Failed to create maildir: " + err.Error())
	}

	// Default calendar and addressbook for groupware.
	db.Exec("INSERT INTO calendars (user_id, name, ctag) VALUES (?, 'Calendar', MD5(NOW()))", id)
	db.Exec("INSERT INTO addressbooks (user_id, name, ctag) VALUES (?, 'Contacts', MD5(NOW()))", id)

	return id, http.StatusCreated, nil
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	_, emailDomain, err := splitAddress(user.Email)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	if user.Quota < 0 {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Quota must be 0 (unlimited) or a positive number of megabytes"})
		return
	}

	var domainName string
	err = db.QueryRow("SELECT name FROM virtual_domains WHERE id = ?", user.DomainID).Scan(&domainName)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Unknown domain_id"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	if !strings.EqualFold(domainName, emailDomain) {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{
			Success: false,
			Message: fmt.Sprintf("Email domain %q does not match domain %q", emailDomain, domainName),
		})
		return
	}

	var oldEmail string
	err = db.QueryRow("SELECT email FROM virtual_users WHERE id = ?", id).Scan(&oldEmail)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "User not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	_, err = db.Exec("UPDATE virtual_users SET email = ?, domain_id = ?, quota = ? WHERE id = ?",
		user.Email, user.DomainID, user.Quota, id)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrDupEntry {
			respondJSON(w, http.StatusConflict, models.APIResponse{Success: false, Message: "Email address already exists"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	// The mailbox path is derived from the address, so a rename has to move the
	// existing mail with it
	if oldEmail != user.Email {
		if err := MoveMaildir(oldEmail, user.Email); err != nil {
			db.Exec("UPDATE virtual_users SET email = ? WHERE id = ?", oldEmail, id)
			respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Failed to move maildir: " + err.Error()})
			return
		}
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "User updated"})
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	_, err = db.Exec("DELETE FROM virtual_users WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "User deleted"})
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	var pc models.PasswordChange
	if err := json.NewDecoder(r.Body).Decode(&pc); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	if pc.Password == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Password required"})
		return
	}

	hashedPassword, err := hashPassword(pc.Password)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Failed to hash password: " + err.Error()})
		return
	}

	_, err = db.Exec("UPDATE virtual_users SET password = ? WHERE id = ?", hashedPassword, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Password changed"})
}

func hashPassword(password string) (string, error) {
	cmd := exec.Command("doveadm", "pw", "-s", "SHA512-CRYPT", "-p", password)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(string(output))
	if hash == "" {
		return "", errors.New("doveadm returned an empty hash")
	}
	return hash, nil
}
