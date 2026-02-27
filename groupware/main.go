package main

import (
	"log"
	"net/http"
	"os"

	"groupware/handlers"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	handlers.InitDB()
	defer handlers.CloseDB()

	r := mux.NewRouter()

	// ==========================================
	// CalDAV/CardDAV Protocol Endpoints (for native clients)
	// ==========================================
	r.PathPrefix("/.well-known/caldav").HandlerFunc(handlers.WellKnownCalDAV)
	r.PathPrefix("/caldav/").HandlerFunc(handlers.CalDAVHandler)
	r.PathPrefix("/.well-known/carddav").HandlerFunc(handlers.WellKnownCardDAV)
	r.PathPrefix("/carddav/").HandlerFunc(handlers.CardDAVHandler)

	// ==========================================
	// REST API Endpoints (for web client)
	// ==========================================

	// Public routes
	r.HandleFunc("/api/login", handlers.Login).Methods("POST")

	// Protected routes (JWT auth)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(handlers.JWTAuthMiddleware)

	// Email - Folders
	api.HandleFunc("/folders", handlers.GetFolders).Methods("GET")

	// Email - Messages
	api.HandleFunc("/messages/{folder}", handlers.GetMessages).Methods("GET")
	api.HandleFunc("/messages/{folder}/{uid}", handlers.GetMessage).Methods("GET")
	api.HandleFunc("/messages/{folder}/{uid}", handlers.DeleteMessage).Methods("DELETE")
	api.HandleFunc("/messages/{folder}/{uid}/move", handlers.MoveMessage).Methods("POST")
	api.HandleFunc("/messages/{folder}/{uid}/flags", handlers.UpdateFlags).Methods("PUT")
	api.HandleFunc("/messages/{folder}/{uid}/attachments/{index}", handlers.GetAttachment).Methods("GET")

	// Email - Send
	api.HandleFunc("/send", handlers.SendMessage).Methods("POST")

	// Personal Calendar
	api.HandleFunc("/calendars", handlers.GetCalendars).Methods("GET")
	api.HandleFunc("/events", handlers.GetEvents).Methods("GET")
	api.HandleFunc("/events", handlers.CreateEvent).Methods("POST")
	api.HandleFunc("/events/{id}", handlers.UpdateEvent).Methods("PUT")
	api.HandleFunc("/events/{id}", handlers.DeleteEvent).Methods("DELETE")

	// Contacts
	api.HandleFunc("/addressbooks", handlers.GetAddressbooks).Methods("GET")
	api.HandleFunc("/contacts", handlers.GetContacts).Methods("GET")
	api.HandleFunc("/contacts", handlers.CreateContact).Methods("POST")
	api.HandleFunc("/contacts/{id}", handlers.UpdateContact).Methods("PUT")
	api.HandleFunc("/contacts/{id}", handlers.DeleteContact).Methods("DELETE")

	// Shared Calendars (Group Calendars)
	api.HandleFunc("/shared-calendars", handlers.GetSharedCalendars).Methods("GET")
	api.HandleFunc("/shared-calendars", handlers.CreateSharedCalendar).Methods("POST")
	api.HandleFunc("/shared-calendars/{id}", handlers.UpdateSharedCalendar).Methods("PUT")
	api.HandleFunc("/shared-calendars/{id}", handlers.DeleteSharedCalendar).Methods("DELETE")
	api.HandleFunc("/shared-calendars/{id}/members", handlers.GetCalendarMembers).Methods("GET")
	api.HandleFunc("/shared-calendars/{id}/members", handlers.AddCalendarMember).Methods("POST")
	api.HandleFunc("/shared-calendars/{id}/members/{memberId}", handlers.RemoveCalendarMember).Methods("DELETE")
	api.HandleFunc("/shared-calendars/{id}/events", handlers.GetSharedEvents).Methods("GET")
	api.HandleFunc("/shared-calendars/{id}/events", handlers.CreateSharedEvent).Methods("POST")
	api.HandleFunc("/shared-calendars/{id}/events/{eventId}", handlers.UpdateSharedEvent).Methods("PUT")
	api.HandleFunc("/shared-calendars/{id}/events/{eventId}", handlers.DeleteSharedEvent).Methods("DELETE")
	api.HandleFunc("/shared-calendars/{id}/freebusy", handlers.GetFreeBusy).Methods("GET")
	api.HandleFunc("/shared-calendars/{id}/find-time", handlers.FindAvailableTime).Methods("POST")
	api.HandleFunc("/domain-users", handlers.GetDomainUsers).Methods("GET")

	// User settings
	api.HandleFunc("/change-password", handlers.ChangePassword).Methods("POST")

	// ==========================================
	// Static Files (Frontend)
	// ==========================================

	// Serve static files from public directory
	publicPath := "./public"
	if _, err := os.Stat(publicPath); err == nil {
		spa := spaHandler{staticPath: publicPath, indexPath: "index.html"}
		r.PathPrefix("/").Handler(spa)
	}

	// ==========================================
	// Basic Auth for CalDAV/CardDAV
	// ==========================================
	basicAuthRouter := handlers.BasicAuthMiddleware(r)

	// CORS configuration
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PROPFIND", "REPORT", "MKCALENDAR"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Depth", "If-Match", "If-None-Match"},
		AllowCredentials: true,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Groupware server starting on port %s", port)
	log.Printf("  - CalDAV: /caldav/")
	log.Printf("  - CardDAV: /carddav/")
	log.Printf("  - REST API: /api/")
	log.Printf("  - Frontend: /")
	log.Fatal(http.ListenAndServe(":"+port, c.Handler(basicAuthRouter)))
}

// spaHandler implements the http.Handler interface for Single Page Applications
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Check if the path is for API or protocol endpoints
	if len(path) >= 4 && path[:4] == "/api" {
		http.NotFound(w, r)
		return
	}
	if len(path) >= 7 && path[:7] == "/caldav" {
		http.NotFound(w, r)
		return
	}
	if len(path) >= 8 && path[:8] == "/carddav" {
		http.NotFound(w, r)
		return
	}

	// Check if file exists
	fullPath := h.staticPath + path
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// File doesn't exist - check if it's a /mail/ SPA route
		if len(path) >= 5 && path[:5] == "/mail" {
			http.ServeFile(w, r, h.staticPath+"/mail/index.html")
			return
		}
		// Otherwise serve root index.html
		http.ServeFile(w, r, h.staticPath+"/"+h.indexPath)
		return
	}

	// File exists, serve it
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}
