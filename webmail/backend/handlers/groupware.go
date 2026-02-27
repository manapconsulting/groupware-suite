package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

var groupwareDB *sql.DB

func InitGroupwareDB() {
	dsn := "mailuser:PostFixCtl8149C?+@tcp(127.0.0.1:3306)/mailserver?parseTime=true"
	var err error
	groupwareDB, err = sql.Open("mysql", dsn)
	if err != nil {
		panic("Failed to connect to groupware database: " + err.Error())
	}
}

func CloseGroupwareDB() {
	if groupwareDB != nil {
		groupwareDB.Close()
	}
}

// Calendar Types
type Calendar struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
}

type Event struct {
	ID          int       `json:"id"`
	CalendarID  int       `json:"calendar_id"`
	UID         string    `json:"uid"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	AllDay      bool      `json:"all_day"`
}

// Contact Types
type Addressbook struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Contact struct {
	ID            int    `json:"id"`
	AddressbookID int    `json:"addressbook_id"`
	UID           string `json:"uid"`
	FullName      string `json:"full_name"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
}

// Helper to get user ID from email
func getUserIDFromEmail(email string) (int, error) {
	var userID int
	err := groupwareDB.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)
	return userID, err
}

// Calendar Handlers
func GetCalendars(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	// Ensure default calendar exists
	var count int
	groupwareDB.QueryRow("SELECT COUNT(*) FROM calendars WHERE user_id = ?", userID).Scan(&count)
	if count == 0 {
		groupwareDB.Exec("INSERT INTO calendars (user_id, name, color, ctag) VALUES (?, 'Takvim', '#3498db', ?)",
			userID, time.Now().Format("20060102150405"))
	}

	rows, err := groupwareDB.Query("SELECT id, name, COALESCE(color, '#3498db'), COALESCE(description, '') FROM calendars WHERE user_id = ?", userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var calendars []Calendar
	for rows.Next() {
		var c Calendar
		rows.Scan(&c.ID, &c.Name, &c.Color, &c.Description)
		calendars = append(calendars, c)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": calendars})
}

func GetEvents(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	// Get optional date range
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var rows *sql.Rows
	if startStr != "" && endStr != "" {
		rows, err = groupwareDB.Query(`
			SELECT e.id, e.calendar_id, e.uid, COALESCE(e.summary, ''), COALESCE(e.description, ''),
				   COALESCE(e.location, ''), e.start_time, e.end_time, COALESCE(e.all_day, 0)
			FROM events e
			JOIN calendars c ON e.calendar_id = c.id
			WHERE c.user_id = ? AND e.start_time >= ? AND e.end_time <= ?
			ORDER BY e.start_time`, userID, startStr, endStr)
	} else {
		rows, err = groupwareDB.Query(`
			SELECT e.id, e.calendar_id, e.uid, COALESCE(e.summary, ''), COALESCE(e.description, ''),
				   COALESCE(e.location, ''), e.start_time, e.end_time, COALESCE(e.all_day, 0)
			FROM events e
			JOIN calendars c ON e.calendar_id = c.id
			WHERE c.user_id = ?
			ORDER BY e.start_time`, userID)
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var allDay int
		var startTime, endTime sql.NullTime
		rows.Scan(&e.ID, &e.CalendarID, &e.UID, &e.Summary, &e.Description, &e.Location, &startTime, &endTime, &allDay)
		if startTime.Valid {
			e.StartTime = startTime.Time
		}
		if endTime.Valid {
			e.EndTime = endTime.Time
		}
		e.AllDay = allDay == 1
		events = append(events, e)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": events})
}

func CreateEvent(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	// Get user's default calendar if not specified
	if event.CalendarID == 0 {
		groupwareDB.QueryRow("SELECT id FROM calendars WHERE user_id = ? LIMIT 1", userID).Scan(&event.CalendarID)
	}

	// Generate UID
	event.UID = time.Now().Format("20060102150405") + "@webmail"

	allDay := 0
	if event.AllDay {
		allDay = 1
	}

	// Generate iCal data
	icalData := generateICalEvent(event)

	result, err := groupwareDB.Exec(`
		INSERT INTO events (calendar_id, uid, summary, description, location, start_time, end_time, all_day, ical_data, etag)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.CalendarID, event.UID, event.Summary, event.Description, event.Location,
		event.StartTime, event.EndTime, allDay, icalData, time.Now().Format("20060102150405"))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	event.ID = int(id)

	respondJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "data": event})
}

func UpdateEvent(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	allDay := 0
	if event.AllDay {
		allDay = 1
	}

	icalData := generateICalEvent(event)

	_, err := groupwareDB.Exec(`
		UPDATE events SET summary = ?, description = ?, location = ?, start_time = ?, end_time = ?, all_day = ?, ical_data = ?, etag = ?
		WHERE id = ?`,
		event.Summary, event.Description, event.Location, event.StartTime, event.EndTime, allDay, icalData,
		time.Now().Format("20060102150405"), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Event updated"})
}

func DeleteEvent(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	_, err := groupwareDB.Exec("DELETE FROM events WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Event deleted"})
}

// Contacts Handlers
func GetAddressbooks(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	// Ensure default addressbook exists
	var count int
	groupwareDB.QueryRow("SELECT COUNT(*) FROM addressbooks WHERE user_id = ?", userID).Scan(&count)
	if count == 0 {
		groupwareDB.Exec("INSERT INTO addressbooks (user_id, name, ctag) VALUES (?, 'Kisiler', ?)",
			userID, time.Now().Format("20060102150405"))
	}

	rows, err := groupwareDB.Query("SELECT id, name, COALESCE(description, '') FROM addressbooks WHERE user_id = ?", userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var addressbooks []Addressbook
	for rows.Next() {
		var a Addressbook
		rows.Scan(&a.ID, &a.Name, &a.Description)
		addressbooks = append(addressbooks, a)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": addressbooks})
}

func GetContacts(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	// Optional search query
	query := r.URL.Query().Get("q")

	var rows *sql.Rows
	if query != "" {
		searchPattern := "%" + query + "%"
		rows, err = groupwareDB.Query(`
			SELECT c.id, c.addressbook_id, c.uid, COALESCE(c.full_name, ''), COALESCE(c.email, ''), COALESCE(c.phone, '')
			FROM contacts c
			JOIN addressbooks a ON c.addressbook_id = a.id
			WHERE a.user_id = ? AND (c.full_name LIKE ? OR c.email LIKE ? OR c.phone LIKE ?)
			ORDER BY c.full_name`, userID, searchPattern, searchPattern, searchPattern)
	} else {
		rows, err = groupwareDB.Query(`
			SELECT c.id, c.addressbook_id, c.uid, COALESCE(c.full_name, ''), COALESCE(c.email, ''), COALESCE(c.phone, '')
			FROM contacts c
			JOIN addressbooks a ON c.addressbook_id = a.id
			WHERE a.user_id = ?
			ORDER BY c.full_name`, userID)
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		rows.Scan(&c.ID, &c.AddressbookID, &c.UID, &c.FullName, &c.Email, &c.Phone)
		contacts = append(contacts, c)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": contacts})
}

func CreateContact(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	var contact Contact
	if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	// Get user's default addressbook if not specified
	if contact.AddressbookID == 0 {
		groupwareDB.QueryRow("SELECT id FROM addressbooks WHERE user_id = ? LIMIT 1", userID).Scan(&contact.AddressbookID)
	}

	// Generate UID
	contact.UID = time.Now().Format("20060102150405") + "@webmail"

	// Generate vCard data
	vcardData := generateVCard(contact)

	result, err := groupwareDB.Exec(`
		INSERT INTO contacts (addressbook_id, uid, full_name, email, phone, vcard_data, etag)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		contact.AddressbookID, contact.UID, contact.FullName, contact.Email, contact.Phone,
		vcardData, time.Now().Format("20060102150405"))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	contact.ID = int(id)

	respondJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "data": contact})
}

func UpdateContact(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var contact Contact
	if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	vcardData := generateVCard(contact)

	_, err := groupwareDB.Exec(`
		UPDATE contacts SET full_name = ?, email = ?, phone = ?, vcard_data = ?, etag = ?
		WHERE id = ?`,
		contact.FullName, contact.Email, contact.Phone, vcardData, time.Now().Format("20060102150405"), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Contact updated"})
}

func DeleteContact(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	_, err := groupwareDB.Exec("DELETE FROM contacts WHERE id = ?", id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Contact deleted"})
}

// Helper functions
func generateICalEvent(e Event) string {
	startFormat := "20060102T150405Z"
	endFormat := "20060102T150405Z"
	if e.AllDay {
		startFormat = "20060102"
		endFormat = "20060102"
	}

	ical := "BEGIN:VCALENDAR\r\n"
	ical += "VERSION:2.0\r\n"
	ical += "PRODID:-//Webmail//EN\r\n"
	ical += "BEGIN:VEVENT\r\n"
	ical += "UID:" + e.UID + "\r\n"
	ical += "DTSTART:" + e.StartTime.UTC().Format(startFormat) + "\r\n"
	ical += "DTEND:" + e.EndTime.UTC().Format(endFormat) + "\r\n"
	ical += "SUMMARY:" + escapeICalText(e.Summary) + "\r\n"
	if e.Description != "" {
		ical += "DESCRIPTION:" + escapeICalText(e.Description) + "\r\n"
	}
	if e.Location != "" {
		ical += "LOCATION:" + escapeICalText(e.Location) + "\r\n"
	}
	ical += "END:VEVENT\r\n"
	ical += "END:VCALENDAR\r\n"
	return ical
}

func generateVCard(c Contact) string {
	vcard := "BEGIN:VCARD\r\n"
	vcard += "VERSION:3.0\r\n"
	vcard += "UID:" + c.UID + "\r\n"
	vcard += "FN:" + c.FullName + "\r\n"
	if c.Email != "" {
		vcard += "EMAIL:" + c.Email + "\r\n"
	}
	if c.Phone != "" {
		vcard += "TEL:" + c.Phone + "\r\n"
	}
	vcard += "END:VCARD\r\n"
	return vcard
}

func escapeICalText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, "\n", "\\n")
	return text
}

// Shared Calendar Types
type SharedCalendar struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Color       string           `json:"color"`
	Description string           `json:"description,omitempty"`
	OwnerID     int              `json:"owner_id"`
	OwnerEmail  string           `json:"owner_email,omitempty"`
	Role        string           `json:"role,omitempty"`
	Members     []CalendarMember `json:"members,omitempty"`
}

type CalendarMember struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type SharedEvent struct {
	ID          int       `json:"id"`
	CalendarID  int       `json:"calendar_id"`
	UID         string    `json:"uid"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	AllDay      bool      `json:"all_day"`
	CreatedBy   int       `json:"created_by"`
	CreatorEmail string   `json:"creator_email,omitempty"`
}

type FreeBusySlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Status    string    `json:"status"` // "busy" or "free"
}

type UserFreeBusy struct {
	UserID int            `json:"user_id"`
	Email  string         `json:"email"`
	Slots  []FreeBusySlot `json:"slots"`
}

type AvailableSlot struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Get Shared Calendars - returns calendars the user owns or is a member of
func GetSharedCalendars(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	// Get calendars where user is owner or member
	rows, err := groupwareDB.Query(`
		SELECT DISTINCT sc.id, sc.name, COALESCE(sc.color, '#9b59b6'), COALESCE(sc.description, ''),
			   sc.owner_id, vu.email as owner_email,
			   COALESCE(cm.role, 'owner') as role
		FROM shared_calendars sc
		LEFT JOIN calendar_members cm ON sc.id = cm.calendar_id AND cm.user_id = ?
		LEFT JOIN virtual_users vu ON sc.owner_id = vu.id
		WHERE sc.owner_id = ? OR cm.user_id = ?
		ORDER BY sc.name`, userID, userID, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var calendars []SharedCalendar
	for rows.Next() {
		var c SharedCalendar
		rows.Scan(&c.ID, &c.Name, &c.Color, &c.Description, &c.OwnerID, &c.OwnerEmail, &c.Role)
		if c.OwnerID == userID {
			c.Role = "owner"
		}
		calendars = append(calendars, c)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": calendars})
}

// Create Shared Calendar
func CreateSharedCalendar(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, err := getUserIDFromEmail(claims.Email)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	var cal SharedCalendar
	if err := json.NewDecoder(r.Body).Decode(&cal); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	if cal.Color == "" {
		cal.Color = "#9b59b6"
	}

	result, err := groupwareDB.Exec(`
		INSERT INTO shared_calendars (name, color, description, owner_id)
		VALUES (?, ?, ?, ?)`,
		cal.Name, cal.Color, cal.Description, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	cal.ID = int(id)
	cal.OwnerID = userID
	cal.Role = "owner"

	respondJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "data": cal})
}

// Update Shared Calendar
func UpdateSharedCalendar(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check ownership
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID != userID {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	var cal SharedCalendar
	if err := json.NewDecoder(r.Body).Decode(&cal); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	_, err := groupwareDB.Exec(`UPDATE shared_calendars SET name = ?, color = ?, description = ? WHERE id = ?`,
		cal.Name, cal.Color, cal.Description, calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Calendar updated"})
}

// Delete Shared Calendar
func DeleteSharedCalendar(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check ownership
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID != userID {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	_, err := groupwareDB.Exec("DELETE FROM shared_calendars WHERE id = ?", calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Calendar deleted"})
}

// Get Calendar Members
func GetCalendarMembers(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	rows, err := groupwareDB.Query(`
		SELECT cm.id, cm.user_id, vu.email, cm.role
		FROM calendar_members cm
		JOIN virtual_users vu ON cm.user_id = vu.id
		WHERE cm.calendar_id = ?
		ORDER BY vu.email`, calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var members []CalendarMember
	for rows.Next() {
		var m CalendarMember
		rows.Scan(&m.ID, &m.UserID, &m.Email, &m.Role)
		members = append(members, m)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": members})
}

// Add Calendar Member
func AddCalendarMember(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check ownership or editor role
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID != userID {
		var role string
		groupwareDB.QueryRow("SELECT role FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&role)
		if role != "editor" {
			respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
			return
		}
	}

	var member struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&member); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	// Find user by email
	var newUserID int
	err := groupwareDB.QueryRow("SELECT id FROM virtual_users WHERE email = ?", member.Email).Scan(&newUserID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": "User not found"})
		return
	}

	if member.Role == "" {
		member.Role = "viewer"
	}

	_, err = groupwareDB.Exec(`
		INSERT INTO calendar_members (calendar_id, user_id, role)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE role = VALUES(role)`,
		calID, newUserID, member.Role)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "message": "Member added"})
}

// Remove Calendar Member
func RemoveCalendarMember(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])
	memberID, _ := strconv.Atoi(vars["memberId"])

	// Check ownership
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID != userID {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	_, err := groupwareDB.Exec("DELETE FROM calendar_members WHERE id = ?", memberID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Member removed"})
}

// Get Shared Events
func GetSharedEvents(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check access
	var hasAccess bool
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID == userID {
		hasAccess = true
	} else {
		var memberCount int
		groupwareDB.QueryRow("SELECT COUNT(*) FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&memberCount)
		hasAccess = memberCount > 0
	}

	if !hasAccess {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	rows, err := groupwareDB.Query(`
		SELECT se.id, se.calendar_id, se.uid, COALESCE(se.summary, ''), COALESCE(se.description, ''),
			   COALESCE(se.location, ''), se.start_time, se.end_time, COALESCE(se.all_day, 0),
			   se.created_by, vu.email
		FROM shared_events se
		LEFT JOIN virtual_users vu ON se.created_by = vu.id
		WHERE se.calendar_id = ?
		ORDER BY se.start_time`, calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var events []SharedEvent
	for rows.Next() {
		var e SharedEvent
		var allDay int
		rows.Scan(&e.ID, &e.CalendarID, &e.UID, &e.Summary, &e.Description, &e.Location,
			&e.StartTime, &e.EndTime, &allDay, &e.CreatedBy, &e.CreatorEmail)
		e.AllDay = allDay == 1
		events = append(events, e)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": events})
}

// Create Shared Event
func CreateSharedEvent(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check edit access
	var canEdit bool
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID == userID {
		canEdit = true
	} else {
		var role string
		groupwareDB.QueryRow("SELECT role FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&role)
		canEdit = role == "editor"
	}

	if !canEdit {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized to add events"})
		return
	}

	var event SharedEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	event.UID = time.Now().Format("20060102150405") + "@webmail-shared"
	allDay := 0
	if event.AllDay {
		allDay = 1
	}

	result, err := groupwareDB.Exec(`
		INSERT INTO shared_events (calendar_id, uid, summary, description, location, start_time, end_time, all_day, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		calID, event.UID, event.Summary, event.Description, event.Location,
		event.StartTime, event.EndTime, allDay, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	event.ID = int(id)
	event.CalendarID = calID
	event.CreatedBy = userID

	respondJSON(w, http.StatusCreated, map[string]interface{}{"success": true, "data": event})
}

// Update Shared Event
func UpdateSharedEvent(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])
	eventID, _ := strconv.Atoi(vars["eventId"])

	// Check edit access
	var canEdit bool
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID == userID {
		canEdit = true
	} else {
		var role string
		groupwareDB.QueryRow("SELECT role FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&role)
		canEdit = role == "editor"
	}

	if !canEdit {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	var event SharedEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	allDay := 0
	if event.AllDay {
		allDay = 1
	}

	_, err := groupwareDB.Exec(`
		UPDATE shared_events SET summary = ?, description = ?, location = ?, start_time = ?, end_time = ?, all_day = ?
		WHERE id = ? AND calendar_id = ?`,
		event.Summary, event.Description, event.Location, event.StartTime, event.EndTime, allDay, eventID, calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Event updated"})
}

// Delete Shared Event
func DeleteSharedEvent(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])
	eventID, _ := strconv.Atoi(vars["eventId"])

	// Check edit access
	var canEdit bool
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID == userID {
		canEdit = true
	} else {
		var role string
		groupwareDB.QueryRow("SELECT role FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&role)
		canEdit = role == "editor"
	}

	if !canEdit {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	_, err := groupwareDB.Exec("DELETE FROM shared_events WHERE id = ? AND calendar_id = ?", eventID, calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Event deleted"})
}

// Get Free/Busy for calendar members
func GetFreeBusy(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check access
	var hasAccess bool
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID == userID {
		hasAccess = true
	} else {
		var memberCount int
		groupwareDB.QueryRow("SELECT COUNT(*) FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&memberCount)
		hasAccess = memberCount > 0
	}

	if !hasAccess {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	// Get date range from query params
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		// Default to current week
		now := time.Now()
		startStr = now.Truncate(24 * time.Hour).Format("2006-01-02")
		endStr = now.AddDate(0, 0, 7).Format("2006-01-02")
	}

	// Get all members (including owner)
	memberRows, err := groupwareDB.Query(`
		SELECT vu.id, vu.email FROM virtual_users vu
		WHERE vu.id = ? OR vu.id IN (
			SELECT user_id FROM calendar_members WHERE calendar_id = ?
		)`, ownerID, calID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer memberRows.Close()

	var result []UserFreeBusy
	for memberRows.Next() {
		var memberID int
		var memberEmail string
		memberRows.Scan(&memberID, &memberEmail)

		// Get events for this user in the date range
		eventRows, _ := groupwareDB.Query(`
			SELECT e.start_time, e.end_time FROM events e
			JOIN calendars c ON e.calendar_id = c.id
			WHERE c.user_id = ? AND e.start_time >= ? AND e.end_time <= ?
			ORDER BY e.start_time`, memberID, startStr, endStr)

		var slots []FreeBusySlot
		for eventRows.Next() {
			var slot FreeBusySlot
			eventRows.Scan(&slot.StartTime, &slot.EndTime)
			slot.Status = "busy"
			slots = append(slots, slot)
		}
		eventRows.Close()

		result = append(result, UserFreeBusy{
			UserID: memberID,
			Email:  memberEmail,
			Slots:  slots,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": result})
}

// Find Available Time - finds common free slots for all members
func FindAvailableTime(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	userID, _ := getUserIDFromEmail(claims.Email)
	vars := mux.Vars(r)
	calID, _ := strconv.Atoi(vars["id"])

	// Check access
	var hasAccess bool
	var ownerID int
	groupwareDB.QueryRow("SELECT owner_id FROM shared_calendars WHERE id = ?", calID).Scan(&ownerID)
	if ownerID == userID {
		hasAccess = true
	} else {
		var memberCount int
		groupwareDB.QueryRow("SELECT COUNT(*) FROM calendar_members WHERE calendar_id = ? AND user_id = ?", calID, userID).Scan(&memberCount)
		hasAccess = memberCount > 0
	}

	if !hasAccess {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "message": "Not authorized"})
		return
	}

	// Parse request
	var req struct {
		StartDate    string `json:"start_date"`
		EndDate      string `json:"end_date"`
		DurationMins int    `json:"duration_mins"`
		WorkdayStart int    `json:"workday_start"` // hour, e.g., 9
		WorkdayEnd   int    `json:"workday_end"`   // hour, e.g., 18
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid request"})
		return
	}

	if req.DurationMins == 0 {
		req.DurationMins = 60
	}
	if req.WorkdayStart == 0 {
		req.WorkdayStart = 9
	}
	if req.WorkdayEnd == 0 {
		req.WorkdayEnd = 18
	}

	startDate, _ := time.Parse("2006-01-02", req.StartDate)
	endDate, _ := time.Parse("2006-01-02", req.EndDate)

	// Get all member IDs
	var memberIDs []int
	memberIDs = append(memberIDs, ownerID)

	memberRows, _ := groupwareDB.Query("SELECT user_id FROM calendar_members WHERE calendar_id = ?", calID)
	for memberRows.Next() {
		var mID int
		memberRows.Scan(&mID)
		memberIDs = append(memberIDs, mID)
	}
	memberRows.Close()

	// Get all busy slots for all members
	allBusySlots := make(map[string]bool) // key: "2006-01-02 15:04"

	for _, memberID := range memberIDs {
		eventRows, _ := groupwareDB.Query(`
			SELECT e.start_time, e.end_time FROM events e
			JOIN calendars c ON e.calendar_id = c.id
			WHERE c.user_id = ? AND e.start_time >= ? AND e.end_time <= ?`,
			memberID, startDate.Format("2006-01-02"), endDate.AddDate(0, 0, 1).Format("2006-01-02"))

		for eventRows.Next() {
			var start, end time.Time
			eventRows.Scan(&start, &end)
			// Mark all 30-minute slots as busy
			for t := start; t.Before(end); t = t.Add(30 * time.Minute) {
				key := t.Format("2006-01-02 15:04")
				allBusySlots[key] = true
			}
		}
		eventRows.Close()
	}

	// Find available slots
	var availableSlots []AvailableSlot
	slotDuration := time.Duration(req.DurationMins) * time.Minute

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		// Skip weekends
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}

		dayStart := time.Date(d.Year(), d.Month(), d.Day(), req.WorkdayStart, 0, 0, 0, d.Location())
		dayEnd := time.Date(d.Year(), d.Month(), d.Day(), req.WorkdayEnd, 0, 0, 0, d.Location())

		for slotStart := dayStart; slotStart.Add(slotDuration).Before(dayEnd) || slotStart.Add(slotDuration).Equal(dayEnd); slotStart = slotStart.Add(30 * time.Minute) {
			// Check if all 30-min intervals in this slot are free
			isFree := true
			for t := slotStart; t.Before(slotStart.Add(slotDuration)); t = t.Add(30 * time.Minute) {
				key := t.Format("2006-01-02 15:04")
				if allBusySlots[key] {
					isFree = false
					break
				}
			}

			if isFree {
				availableSlots = append(availableSlots, AvailableSlot{
					StartTime: slotStart,
					EndTime:   slotStart.Add(slotDuration),
				})
			}
		}
	}

	// Limit results
	if len(availableSlots) > 20 {
		availableSlots = availableSlots[:20]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": availableSlots})
}

// Get domain users for member selection
func GetDomainUsers(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "Unauthorized"})
		return
	}

	// Extract domain from user's email
	parts := strings.Split(claims.Email, "@")
	if len(parts) != 2 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid email"})
		return
	}
	domain := parts[1]

	rows, err := groupwareDB.Query(`
		SELECT vu.id, vu.email FROM virtual_users vu
		JOIN virtual_domains vd ON vu.domain_id = vd.id
		WHERE vd.name = ?
		ORDER BY vu.email`, domain)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id int
		var email string
		rows.Scan(&id, &email)
		users = append(users, map[string]interface{}{"id": id, "email": email})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": users})
}
