package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap/client"
	"github.com/golang-jwt/jwt/v5"
)

// tlsConfig for local IMAP connection (skip verify for localhost)
var imapTLSConfig = &tls.Config{
	InsecureSkipVerify: true,
}

var jwtSecret = []byte(getJWTSecret())

func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "webmail-secret-key-change-in-production"
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

	// Call mailadmin API to check if webmail is enabled
	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/api/webmail-check/%s", domain))
	if err != nil {
		// If mailadmin is unreachable, allow login (fail open for now)
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
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	// Check if webmail is enabled for this domain
	enabled, err := checkWebmailEnabled(req.Email)
	if err != nil {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"message": "Domain not found",
		})
		return
	}
	if !enabled {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"message": "Webmail is disabled for this domain",
		})
		return
	}

	// Verify credentials against IMAP server
	c, err := client.DialTLS("localhost:993", imapTLSConfig)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Mail server connection failed",
		})
		return
	}
	defer c.Logout()

	if err := c.Login(req.Email, req.Password); err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Invalid email or password",
		})
		return
	}

	// Create JWT token with credentials (needed for IMAP operations)
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
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Token generation failed",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": LoginResponse{
			Token: tokenString,
			Email: req.Email,
		},
	})
}

func AuthMiddleware(next http.Handler) http.Handler {
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
			respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
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
			respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
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

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
