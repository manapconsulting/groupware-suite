package handlers

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

func GetUsers(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT u.id, u.domain_id, u.email, d.name as domain
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
		if err := rows.Scan(&u.ID, &u.DomainID, &u.Email, &u.Domain); err != nil {
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

	if user.Password == "" {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Password required"})
		return
	}

	// Keep plain password for email before hashing
	plainPassword := user.Password
	hashedPassword := hashPassword(user.Password)

	result, err := db.Exec(
		"INSERT INTO virtual_users (domain_id, email, password) VALUES (?, ?, ?)",
		user.DomainID, user.Email, hashedPassword,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	user.ID = int(id)

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

	_, err = db.Exec("UPDATE virtual_users SET email = ?, domain_id = ? WHERE id = ?",
		user.Email, user.DomainID, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
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

	hashedPassword := hashPassword(pc.Password)

	_, err = db.Exec("UPDATE virtual_users SET password = ? WHERE id = ?", hashedPassword, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Password changed"})
}

func hashPassword(password string) string {
	cmd := exec.Command("doveadm", "pw", "-s", "SHA512-CRYPT", "-p", password)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
