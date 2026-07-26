package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mailadmin/models"

	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

// The Provision* handlers sit behind APIKeyMiddleware. Every operation is
// scoped to the domain the calling key is bound to, taken from the request
// context -- never from the request body or URL. A key for vendisens.com can
// therefore only ever touch vendisens.com mailboxes.

// ownedUserEmail returns the address of user id, but only if it belongs to
// domainID. This is the ownership gate for update/delete/password by key.
func ownedUserEmail(id, domainID int) (string, bool) {
	var email string
	err := db.QueryRow("SELECT email FROM virtual_users WHERE id = ? AND domain_id = ?", id, domainID).Scan(&email)
	if err != nil {
		return "", false
	}
	return email, true
}

// ProvisionCreateUser creates a mailbox for the key's domain.
func ProvisionCreateUser(w http.ResponseWriter, r *http.Request) {
	domainID, domain, ok := keyDomain(r)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Missing key context"})
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
	if !strings.EqualFold(emailDomain, domain) {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{
			Success: false,
			Message: fmt.Sprintf("This key may only manage %q addresses, not %q", domain, emailDomain),
		})
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

	// Force the domain from the key; ignore anything the caller supplied.
	user.DomainID = domainID
	plainPassword := user.Password

	id, status, err := insertMailbox(&user)
	if err != nil {
		respondJSON(w, status, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	user.ID = id

	if user.NotifyEmail != "" {
		notifyEmail := user.NotifyEmail
		emailData := WelcomeEmailData{
			Email:    user.Email,
			Password: plainPassword,
			Domain:   ExtractDomain(user.Email),
		}
		go func() {
			if err := SendWelcomeEmail(notifyEmail, emailData); err != nil {
				println("Failed to send welcome email:", err.Error())
			}
		}()
	}

	user.Password = ""
	user.NotifyEmail = ""
	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: user})
}

// ProvisionListUsers lists mailboxes belonging to the key's domain.
func ProvisionListUsers(w http.ResponseWriter, r *http.Request) {
	domainID, _, ok := keyDomain(r)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Missing key context"})
		return
	}

	rows, err := db.Query(
		`SELECT u.id, u.domain_id, u.email, COALESCE(u.quota, 0), d.name
		 FROM virtual_users u JOIN virtual_domains d ON u.domain_id = d.id
		 WHERE u.domain_id = ? ORDER BY u.email`,
		domainID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.DomainID, &u.Email, &u.Quota, &u.Domain); err != nil {
			continue
		}
		users = append(users, u)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: users})
}

// ProvisionUpdateUser renames a mailbox or changes its quota, staying within the
// key's domain. The mailbox may not be moved to another domain.
func ProvisionUpdateUser(w http.ResponseWriter, r *http.Request) {
	domainID, domain, ok := keyDomain(r)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Missing key context"})
		return
	}
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	oldEmail, owned := ownedUserEmail(id, domainID)
	if !owned {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "User not found"})
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
	if !strings.EqualFold(emailDomain, domain) {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{
			Success: false,
			Message: fmt.Sprintf("This key may only manage %q addresses, not %q", domain, emailDomain),
		})
		return
	}
	if user.Quota < 0 {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Quota must be 0 (unlimited) or a positive number of megabytes"})
		return
	}

	result, err := db.Exec(
		"UPDATE virtual_users SET email = ?, quota = ? WHERE id = ? AND domain_id = ?",
		user.Email, user.Quota, id, domainID,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrDupEntry {
			respondJSON(w, http.StatusConflict, models.APIResponse{Success: false, Message: "Email address already exists"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "User not found"})
		return
	}

	if oldEmail != user.Email {
		if err := MoveMaildir(oldEmail, user.Email); err != nil {
			db.Exec("UPDATE virtual_users SET email = ? WHERE id = ?", oldEmail, id)
			respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Failed to move maildir: " + err.Error()})
			return
		}
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "User updated"})
}

// ProvisionDeleteUser removes a mailbox belonging to the key's domain. As with
// the admin path, the Maildir on disk is intentionally left in place.
func ProvisionDeleteUser(w http.ResponseWriter, r *http.Request) {
	domainID, _, ok := keyDomain(r)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Missing key context"})
		return
	}
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	result, err := db.Exec("DELETE FROM virtual_users WHERE id = ? AND domain_id = ?", id, domainID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "User not found"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "User deleted"})
}

// ProvisionChangePassword sets a new password for a mailbox in the key's domain.
func ProvisionChangePassword(w http.ResponseWriter, r *http.Request) {
	domainID, _, ok := keyDomain(r)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Missing key context"})
		return
	}
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	if _, owned := ownedUserEmail(id, domainID); !owned {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "User not found"})
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

	if _, err = db.Exec("UPDATE virtual_users SET password = ? WHERE id = ? AND domain_id = ?", hashedPassword, id, domainID); err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Password changed"})
}
