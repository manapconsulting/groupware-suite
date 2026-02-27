package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

func GetSenderPermissions(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, send_as, login_user FROM sender_permissions ORDER BY send_as")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer rows.Close()

	var permissions []models.SenderPermission
	for rows.Next() {
		var p models.SenderPermission
		if err := rows.Scan(&p.ID, &p.SendAs, &p.LoginUser); err != nil {
			continue
		}
		permissions = append(permissions, p)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: permissions})
}

func CreateSenderPermission(w http.ResponseWriter, r *http.Request) {
	var perm models.SenderPermission
	if err := json.NewDecoder(r.Body).Decode(&perm); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	result, err := db.Exec(
		"INSERT INTO sender_permissions (send_as, login_user) VALUES (?, ?)",
		perm.SendAs, perm.LoginUser,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	perm.ID = int(id)

	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: perm})
}

func DeleteSenderPermission(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid ID"})
		return
	}

	_, err = db.Exec("DELETE FROM sender_permissions WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Permission deleted"})
}
