package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

func GetAliases(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT a.id, a.domain_id, a.source, a.destination, d.name as domain
		FROM virtual_aliases a
		JOIN virtual_domains d ON a.domain_id = d.id
		ORDER BY a.source
	`
	rows, err := db.Query(query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer rows.Close()

	var aliases []models.Alias
	for rows.Next() {
		var a models.Alias
		if err := rows.Scan(&a.ID, &a.DomainID, &a.Source, &a.Destination, &a.Domain); err != nil {
			continue
		}
		aliases = append(aliases, a)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: aliases})
}

func CreateAlias(w http.ResponseWriter, r *http.Request) {
	var alias models.Alias
	if err := json.NewDecoder(r.Body).Decode(&alias); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	result, err := db.Exec(
		"INSERT INTO virtual_aliases (domain_id, source, destination) VALUES (?, ?, ?)",
		alias.DomainID, alias.Source, alias.Destination,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	alias.ID = int(id)

	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: alias})
}

func UpdateAlias(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	var alias models.Alias
	if err := json.NewDecoder(r.Body).Decode(&alias); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	_, err = db.Exec("UPDATE virtual_aliases SET source = ?, destination = ?, domain_id = ? WHERE id = ?",
		alias.Source, alias.Destination, alias.DomainID, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Alias updated"})
}

func DeleteAlias(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	_, err = db.Exec("DELETE FROM virtual_aliases WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Alias deleted"})
}
