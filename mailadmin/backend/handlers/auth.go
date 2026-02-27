package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"mailadmin/middleware"
	"mailadmin/models"

	"github.com/golang-jwt/jwt/v5"
)

var adminUsers map[string]string

func init() {
	adminUsers = make(map[string]string)
	// Format: ADMIN_USERS="admin:password,user2:password2"
	usersEnv := os.Getenv("ADMIN_USERS")
	if usersEnv == "" {
		log.Fatal("ADMIN_USERS environment variable is required (format: user:password,user2:password2)")
	}
	for _, pair := range strings.Split(usersEnv, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			adminUsers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if len(adminUsers) == 0 {
		log.Fatal("No valid admin users configured in ADMIN_USERS")
	}
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	password, exists := adminUsers[req.Username]
	if !exists || password != req.Password {
		respondJSON(w, http.StatusUnauthorized, models.APIResponse{Success: false, Message: "Invalid credentials"})
		return
	}

	claims := &middleware.Claims{
		Username: req.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(middleware.JWTSecret)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Failed to generate token"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    models.LoginResponse{Token: tokenString},
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
