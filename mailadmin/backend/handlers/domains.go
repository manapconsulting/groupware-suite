package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

func GetDomains(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, COALESCE(webmail_enabled, 1), COALESCE(caldav_enabled, 1), COALESCE(carddav_enabled, 1) FROM virtual_domains ORDER BY name")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer rows.Close()

	var domains []models.Domain
	for rows.Next() {
		var d models.Domain
		var webmailEnabled, caldavEnabled, carddavEnabled int
		if err := rows.Scan(&d.ID, &d.Name, &webmailEnabled, &caldavEnabled, &carddavEnabled); err != nil {
			continue
		}
		d.WebmailEnabled = webmailEnabled == 1
		d.CaldavEnabled = caldavEnabled == 1
		d.CarddavEnabled = carddavEnabled == 1
		domains = append(domains, d)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: domains})
}

func CreateDomain(w http.ResponseWriter, r *http.Request) {
	var domain models.Domain
	if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	// Default all services to enabled
	webmailEnabled := boolToInt(domain.WebmailEnabled)
	caldavEnabled := boolToInt(domain.CaldavEnabled)
	carddavEnabled := boolToInt(domain.CarddavEnabled)

	// If not specified, default to enabled
	if !domain.WebmailEnabled && !domain.CaldavEnabled && !domain.CarddavEnabled {
		webmailEnabled, caldavEnabled, carddavEnabled = 1, 1, 1
	}

	result, err := db.Exec("INSERT INTO virtual_domains (name, webmail_enabled, caldav_enabled, carddav_enabled) VALUES (?, ?, ?, ?)",
		domain.Name, webmailEnabled, caldavEnabled, carddavEnabled)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	domain.ID = int(id)
	domain.WebmailEnabled = webmailEnabled == 1
	domain.CaldavEnabled = caldavEnabled == 1
	domain.CarddavEnabled = carddavEnabled == 1

	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: domain})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func UpdateDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	var domain models.Domain
	if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	_, err = db.Exec(`UPDATE virtual_domains SET
		webmail_enabled = ?,
		caldav_enabled = ?,
		carddav_enabled = ?
		WHERE id = ?`,
		boolToInt(domain.WebmailEnabled),
		boolToInt(domain.CaldavEnabled),
		boolToInt(domain.CarddavEnabled),
		id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Domain updated"})
}

func DeleteDomain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	// Check if domain has users
	var count int
	db.QueryRow("SELECT COUNT(*) FROM virtual_users WHERE domain_id = ?", id).Scan(&count)
	if count > 0 {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Domain has users, delete them first"})
		return
	}

	_, err = db.Exec("DELETE FROM virtual_domains WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Domain deleted"})
}

// CheckWebmailEnabled - Public endpoint for webmail to check if domain has webmail enabled
func CheckWebmailEnabled(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	// Extract domain from email if full email provided
	if strings.Contains(domain, "@") {
		parts := strings.Split(domain, "@")
		if len(parts) == 2 {
			domain = parts[1]
		}
	}

	var webmailEnabled int
	err := db.QueryRow("SELECT COALESCE(webmail_enabled, 1) FROM virtual_domains WHERE name = ?", domain).Scan(&webmailEnabled)
	if err != nil {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "Domain not found"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"domain":          domain,
			"webmail_enabled": webmailEnabled == 1,
		},
	})
}

// CheckGroupwareEnabled - Public endpoint for groupware to check if domain has caldav/carddav enabled
func CheckGroupwareEnabled(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	// Extract domain from email if full email provided
	if strings.Contains(domain, "@") {
		parts := strings.Split(domain, "@")
		if len(parts) == 2 {
			domain = parts[1]
		}
	}

	var caldavEnabled, carddavEnabled int
	err := db.QueryRow("SELECT COALESCE(caldav_enabled, 1), COALESCE(carddav_enabled, 1) FROM virtual_domains WHERE name = ?", domain).Scan(&caldavEnabled, &carddavEnabled)
	if err != nil {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "Domain not found"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"domain":          domain,
			"caldav_enabled":  caldavEnabled == 1,
			"carddav_enabled": carddavEnabled == 1,
		},
	})
}
