package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap/client"
	"github.com/golang-jwt/jwt/v5"
)

// TLS config for local IMAP connection
var imapTLSConfig = &tls.Config{
	InsecureSkipVerify: true,
}

var jwtSecret = []byte(getJWTSecret())

func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	return secret
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type Claims struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	jwt.RegisteredClaims
}

type contextKey string

const userContextKey contextKey = "user"

// checkWebmailEnabled checks if webmail is enabled for the given email's domain
func checkWebmailEnabled(email string) (bool, error) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid email format")
	}
	domain := parts[1]

	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/api/webmail-check/%s", domain))
	if err != nil {
		return true, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, fmt.Errorf("domain not found")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, nil
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			WebmailEnabled bool `json:"webmail_enabled"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return true, nil
	}

	return result.Data.WebmailEnabled, nil
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Login JSON decode error: %v", err)
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}
	log.Printf("Login attempt for: %s", req.Email)

	// Check if webmail is enabled for this domain
	enabled, err := checkWebmailEnabled(req.Email)
	if err != nil {
		RespondJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"message": "Domain not found",
		})
		return
	}
	if !enabled {
		RespondJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"message": "Webmail is disabled for this domain",
		})
		return
	}

	// Verify credentials against IMAP server
	c, err := client.DialTLS("localhost:993", imapTLSConfig)
	if err != nil {
		log.Printf("IMAP connection failed: %v", err)
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Mail server connection failed",
		})
		return
	}
	defer c.Logout()

	if err := c.Login(req.Email, req.Password); err != nil {
		log.Printf("IMAP login failed for %s: %v", req.Email, err)
		RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Invalid email or password",
		})
		return
	}
	log.Printf("IMAP login successful for %s", req.Email)

	// Create JWT token with credentials
	claims := &Claims{
		Email:    req.Email,
		Password: req.Password,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Token generation failed",
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": LoginResponse{
			Token: tokenString,
			Email: req.Email,
		},
	})
}

func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// Try Authorization header first
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// Fall back to query parameter (for attachments)
			tokenString = r.URL.Query().Get("token")
		}

		if tokenString == "" {
			RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"message": "Authorization required",
			})
			return
		}

		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"message": "Invalid token",
			})
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserFromContext(r *http.Request) *Claims {
	claims, ok := r.Context().Value(userContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Not authenticated",
		})
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	// Validate new password
	if len(req.NewPassword) < 6 {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Password must be at least 6 characters",
		})
		return
	}

	// Verify current password by trying IMAP login
	c, err := client.DialTLS("localhost:993", imapTLSConfig)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Mail server connection failed",
		})
		return
	}
	defer c.Logout()

	if err := c.Login(claims.Email, req.CurrentPassword); err != nil {
		RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Current password is incorrect",
		})
		return
	}

	// Update password in database
	if err := updateUserPassword(claims.Email, req.NewPassword); err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to update password",
		})
		return
	}

	// Generate new JWT token with new password
	newClaims := &Claims{
		Email:    claims.Email,
		Password: req.NewPassword,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Token generation failed",
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
		"data": map[string]interface{}{
			"token": tokenString,
		},
	})
}
