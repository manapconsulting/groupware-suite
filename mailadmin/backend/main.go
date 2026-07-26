package main

import (
	"log"
	"net/http"
	"os"

	"mailadmin/handlers"
	"mailadmin/middleware"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	handlers.InitDB()
	defer handlers.CloseDB()

	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/api/login", handlers.Login).Methods("POST")
	r.HandleFunc("/api/webmail-check/{domain}", handlers.CheckWebmailEnabled).Methods("GET")
	r.HandleFunc("/api/groupware-check/{domain}", handlers.CheckGroupwareEnabled).Methods("GET")

	// Protected routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware)

	// Domains
	api.HandleFunc("/domains", handlers.GetDomains).Methods("GET")
	api.HandleFunc("/domains", handlers.CreateDomain).Methods("POST")
	api.HandleFunc("/domains/{id}", handlers.UpdateDomain).Methods("PUT")
	api.HandleFunc("/domains/{id}", handlers.DeleteDomain).Methods("DELETE")

	// Per-domain API keys (admin-managed)
	api.HandleFunc("/domains/{id}/api-keys", handlers.GetDomainAPIKeys).Methods("GET")
	api.HandleFunc("/domains/{id}/api-keys", handlers.CreateDomainAPIKey).Methods("POST")
	api.HandleFunc("/domains/{id}/api-keys/{keyId}", handlers.UpdateDomainAPIKey).Methods("PUT")
	api.HandleFunc("/domains/{id}/api-keys/{keyId}", handlers.DeleteDomainAPIKey).Methods("DELETE")

	// DNS Checks
	api.HandleFunc("/dns-check/{domain}", handlers.CheckDomainDNS).Methods("GET")
	api.HandleFunc("/dkim-key/{domain}", handlers.GetDKIMKey).Methods("GET")
	api.HandleFunc("/client-setup/{domain}", handlers.GetClientSetup).Methods("GET")

	// Users
	api.HandleFunc("/users", handlers.GetUsers).Methods("GET")
	api.HandleFunc("/users", handlers.CreateUser).Methods("POST")
	api.HandleFunc("/users/{id}", handlers.UpdateUser).Methods("PUT")
	api.HandleFunc("/users/{id}", handlers.DeleteUser).Methods("DELETE")
	api.HandleFunc("/users/{id}/password", handlers.ChangePassword).Methods("PUT")

	// Aliases
	api.HandleFunc("/aliases", handlers.GetAliases).Methods("GET")
	api.HandleFunc("/aliases", handlers.CreateAlias).Methods("POST")
	api.HandleFunc("/aliases/{id}", handlers.UpdateAlias).Methods("PUT")
	api.HandleFunc("/aliases/{id}", handlers.DeleteAlias).Methods("DELETE")

	// Sender Permissions
	api.HandleFunc("/sender-permissions", handlers.GetSenderPermissions).Methods("GET")
	api.HandleFunc("/sender-permissions", handlers.CreateSenderPermission).Methods("POST")
	api.HandleFunc("/sender-permissions/{id}", handlers.DeleteSenderPermission).Methods("DELETE")

	// Mailing Lists
	api.HandleFunc("/lists", handlers.GetMailingLists).Methods("GET")
	api.HandleFunc("/lists", handlers.CreateMailingList).Methods("POST")
	api.HandleFunc("/lists/{id}", handlers.DeleteMailingList).Methods("DELETE")
	api.HandleFunc("/lists/{id}/members", handlers.GetMailingListMembers).Methods("GET")
	api.HandleFunc("/lists/{id}/members", handlers.AddMailingListMember).Methods("POST")
	api.HandleFunc("/lists/{id}/members/{email}", handlers.RemoveMailingListMember).Methods("DELETE")
	api.HandleFunc("/lists/{id}/settings", handlers.GetMailingListSettings).Methods("GET")
	api.HandleFunc("/lists/{id}/settings", handlers.UpdateMailingListSettings).Methods("PUT")

	// SSL Certificate Management
	api.HandleFunc("/ssl", handlers.GetAllSSLStatus).Methods("GET")
	api.HandleFunc("/ssl/{domain}", handlers.GetSSLStatus).Methods("GET")
	api.HandleFunc("/ssl/configure", handlers.ConfigureSSL).Methods("POST")
	api.HandleFunc("/ssl/webmail", handlers.ConfigureNginxWebmail).Methods("POST")

	// Provisioning routes: authenticated by a per-domain API key
	// (X-API-Key header) and restricted to the key's allowlisted source.
	// Every operation is scoped to the key's own domain.
	provision := r.PathPrefix("/api/provision").Subrouter()
	provision.Use(handlers.APIKeyMiddleware)
	provision.HandleFunc("/users", handlers.ProvisionListUsers).Methods("GET")
	provision.HandleFunc("/users", handlers.ProvisionCreateUser).Methods("POST")
	provision.HandleFunc("/users/{id}", handlers.ProvisionUpdateUser).Methods("PUT")
	provision.HandleFunc("/users/{id}", handlers.ProvisionDeleteUser).Methods("DELETE")
	provision.HandleFunc("/users/{id}/password", handlers.ProvisionChangePassword).Methods("PUT")

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Mail Admin API starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, c.Handler(r)))
}
