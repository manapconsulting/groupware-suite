package main

import (
	"log"
	"net/http"
	"os"

	"webmail/handlers"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Initialize groupware database
	handlers.InitGroupwareDB()
	defer handlers.CloseGroupwareDB()

	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/api/login", handlers.Login).Methods("POST")

	// Protected routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(handlers.AuthMiddleware)

	// Folders
	api.HandleFunc("/folders", handlers.GetFolders).Methods("GET")

	// Messages
	api.HandleFunc("/messages/{folder}", handlers.GetMessages).Methods("GET")
	api.HandleFunc("/messages/{folder}/{uid}", handlers.GetMessage).Methods("GET")
	api.HandleFunc("/messages/{folder}/{uid}", handlers.DeleteMessage).Methods("DELETE")
	api.HandleFunc("/messages/{folder}/{uid}/move", handlers.MoveMessage).Methods("POST")
	api.HandleFunc("/messages/{folder}/{uid}/flags", handlers.UpdateFlags).Methods("PUT")

	// Send
	api.HandleFunc("/send", handlers.SendMessage).Methods("POST")

	// Attachments
	api.HandleFunc("/messages/{folder}/{uid}/attachments/{index}", handlers.GetAttachment).Methods("GET")

	// Calendar
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

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Webmail API starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, c.Handler(r)))
}
