package handlers

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func generateETag() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))
}

func WellKnownCalDAV(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/caldav/", http.StatusMovedPermanently)
}

func CalDAVHandler(w http.ResponseWriter, r *http.Request) {
	email := r.Header.Get("X-User-Email")
	if email == "" {
		// Try to get from basic auth
		email = getUserEmailFromPath(r.URL.Path)
	}

	switch r.Method {
	case "OPTIONS":
		handleCalDAVOptions(w, r)
	case "PROPFIND":
		handleCalDAVPropfind(w, r, email)
	case "REPORT":
		handleCalDAVReport(w, r, email)
	case "PUT":
		handleCalDAVPut(w, r, email)
	case "GET":
		handleCalDAVGet(w, r, email)
	case "DELETE":
		handleCalDAVDelete(w, r, email)
	case "MKCALENDAR":
		handleMkCalendar(w, r, email)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleCalDAVOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "OPTIONS, GET, PUT, DELETE, PROPFIND, REPORT, MKCALENDAR")
	w.Header().Set("DAV", "1, 2, 3, calendar-access")
	w.WriteHeader(http.StatusOK)
}

func handleCalDAVPropfind(w http.ResponseWriter, r *http.Request, email string) {
	path := r.URL.Path

	var userID int
	db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)
	if userID == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "1"
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207) // Multi-Status

	// Parse path to determine what to return
	// Path examples:
	// /caldav/ - principal
	// /caldav/{email}/ - calendar home
	// /caldav/{email}/calendar/ - default calendar collection
	// /caldav/{email}/{calendarID}/ - specific calendar collection
	// /caldav/{email}/calendar/{uid}.ics - event

	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if path == "/caldav/" || path == "/caldav" {
		// Return user principal
		writeCalDAVPrincipal(w, email)
	} else if strings.Contains(path, ".ics") {
		// Return event
		writeEventPropfind(w, email, path)
	} else if len(pathParts) >= 3 {
		// Path like /caldav/{email}/{calendar}/ - return calendar collection
		writeCalendarCollection(w, email, userID)
	} else {
		// Path like /caldav/{email}/ - calendar home
		writeCalendarHome(w, email, userID, depth)
	}
}

func writeCalDAVPrincipal(w http.ResponseWriter, email string) {
	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/caldav/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
        </D:resourcetype>
        <D:current-user-principal>
          <D:href>/caldav/%s/</D:href>
        </D:current-user-principal>
        <C:calendar-home-set>
          <D:href>/caldav/%s/</D:href>
        </C:calendar-home-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, email, email)
	w.Write([]byte(response))
}

func writeCalendarHome(w http.ResponseWriter, email string, userID int, depth string) {
	var responses strings.Builder

	// Add home collection with principal info
	responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/caldav/%s/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
        <D:displayname>%s</D:displayname>
        <D:current-user-principal>
          <D:href>/caldav/%s/</D:href>
        </D:current-user-principal>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, email, email))

	if depth != "0" {
		// List calendars - use "calendar" as the path for better client compatibility
		rows, _ := db.Query("SELECT id, name, ctag FROM calendars WHERE user_id = ? ORDER BY id LIMIT 10", userID)
		defer rows.Close()

		calNum := 0
		for rows.Next() {
			var id int
			var name, ctag string
			rows.Scan(&id, &name, &ctag)

			// Use "calendar" path for the first, "calendar2", "calendar3" etc for others
			calPath := "calendar"
			if calNum > 0 {
				calPath = fmt.Sprintf("calendar%d", calNum+1)
			}
			calNum++

			responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/caldav/%s/%s/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <D:displayname>%s</D:displayname>
        <CS:getctag xmlns:CS="http://calendarserver.org/ns/">%s</CS:getctag>
        <C:supported-calendar-component-set>
          <C:comp name="VEVENT"/>
        </C:supported-calendar-component-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, calPath, name, ctag))
		}
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func writeCalendarCollection(w http.ResponseWriter, email string, userID int) {
	// Get the first calendar for this user
	var calID int
	var calName, calCtag string
	db.QueryRow("SELECT id, name, ctag FROM calendars WHERE user_id = ? ORDER BY id LIMIT 1", userID).Scan(&calID, &calName, &calCtag)

	var responses strings.Builder

	// First, return the calendar collection itself
	responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/caldav/%s/calendar/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <D:displayname>%s</D:displayname>
        <CS:getctag xmlns:CS="http://calendarserver.org/ns/">%s</CS:getctag>
        <C:supported-calendar-component-set>
          <C:comp name="VEVENT"/>
        </C:supported-calendar-component-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, calName, calCtag))

	// Then list all events in this calendar
	rows, _ := db.Query(`
		SELECT e.uid, e.etag
		FROM events e
		WHERE e.calendar_id = ?`, calID)
	defer rows.Close()

	for rows.Next() {
		var uid, etag string
		rows.Scan(&uid, &etag)

		responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/caldav/%s/calendar/%s.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"%s"</D:getetag>
        <D:getcontenttype>text/calendar; charset=utf-8</D:getcontenttype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, uid, etag))
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func writeEventPropfind(w http.ResponseWriter, email string, path string) {
	// Extract UID from path
	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".ics") {
			uid = strings.TrimSuffix(p, ".ics")
			break
		}
	}

	var etag string
	err := db.QueryRow(`
		SELECT e.etag FROM events e
		JOIN calendars c ON e.calendar_id = c.id
		JOIN virtual_users u ON c.user_id = u.id
		WHERE u.email = ? AND e.uid = ?`, email, uid).Scan(&etag)

	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>%s</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"%s"</D:getetag>
        <D:getcontenttype>text/calendar; charset=utf-8</D:getcontenttype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, path, etag)
	w.Write([]byte(response))
}

func handleCalDAVReport(w http.ResponseWriter, r *http.Request, email string) {
	var userID int
	db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)

	body, _ := io.ReadAll(r.Body)
	bodyStr := string(body)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207)

	// Check if it's a calendar-multiget or calendar-query
	if strings.Contains(bodyStr, "calendar-multiget") {
		handleCalendarMultiget(w, email, userID, bodyStr)
	} else {
		handleCalendarQuery(w, email, userID)
	}
}

func handleCalendarMultiget(w http.ResponseWriter, email string, userID int, body string) {
	var responses strings.Builder

	rows, _ := db.Query(`
		SELECT e.uid, e.etag, e.ical_data
		FROM events e
		JOIN calendars c ON e.calendar_id = c.id
		WHERE c.user_id = ?`, userID)
	defer rows.Close()

	for rows.Next() {
		var uid, etag, icalData string
		rows.Scan(&uid, &etag, &icalData)

		responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/caldav/%s/calendar/%s.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"%s"</D:getetag>
        <C:calendar-data>%s</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, uid, etag, xmlEscape(icalData)))
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func handleCalendarQuery(w http.ResponseWriter, email string, userID int) {
	rows, _ := db.Query(`
		SELECT e.uid, e.etag, e.ical_data
		FROM events e
		JOIN calendars c ON e.calendar_id = c.id
		WHERE c.user_id = ?`, userID)
	defer rows.Close()

	var responses strings.Builder
	for rows.Next() {
		var uid, etag, icalData string
		rows.Scan(&uid, &etag, &icalData)

		responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/caldav/%s/calendar/%s.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"%s"</D:getetag>
        <C:calendar-data>%s</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, uid, etag, xmlEscape(icalData)))
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func handleCalDAVPut(w http.ResponseWriter, r *http.Request, email string) {
	var userID int
	db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)
	if userID == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get calendar ID (use first calendar for simplicity)
	var calendarID int
	db.QueryRow("SELECT id FROM calendars WHERE user_id = ? ORDER BY id LIMIT 1", userID).Scan(&calendarID)
	if calendarID == 0 {
		http.Error(w, "Calendar not found", http.StatusNotFound)
		return
	}

	// Extract UID from path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".ics") {
			uid = strings.TrimSuffix(p, ".ics")
			break
		}
	}

	if uid == "" {
		http.Error(w, "Invalid event path", http.StatusBadRequest)
		return
	}

	// Read iCal data
	body, _ := io.ReadAll(r.Body)
	icalData := string(body)

	// Parse fields from iCal
	summary := extractICalField(icalData, "SUMMARY")
	description := extractICalField(icalData, "DESCRIPTION")
	location := extractICalField(icalData, "LOCATION")
	dtstart := extractICalField(icalData, "DTSTART")
	dtend := extractICalField(icalData, "DTEND")

	// Parse dates
	startTime := parseICalDate(dtstart)
	endTime := parseICalDate(dtend)

	etag := generateETag()

	// Insert or update event
	_, err := db.Exec(`
		INSERT INTO events (calendar_id, uid, summary, description, location, start_time, end_time, ical_data, etag)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		summary = VALUES(summary),
		description = VALUES(description),
		location = VALUES(location),
		start_time = VALUES(start_time),
		end_time = VALUES(end_time),
		ical_data = VALUES(ical_data),
		etag = VALUES(etag)`,
		calendarID, uid, summary, description, location, startTime, endTime, icalData, etag)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update calendar ctag
	db.Exec("UPDATE calendars SET ctag = ? WHERE id = ?", generateETag(), calendarID)

	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.WriteHeader(http.StatusCreated)
}

func parseICalDate(dtValue string) *time.Time {
	if dtValue == "" {
		return nil
	}

	// Try different date formats
	formats := []string{
		"20060102T150405Z",     // Basic format with UTC
		"20060102T150405",      // Basic format local
		"20060102",             // Date only
		"2006-01-02T15:04:05Z", // ISO format
		"2006-01-02T15:04:05",  // ISO format local
		"2006-01-02",           // ISO date only
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dtValue); err == nil {
			return &t
		}
	}
	return nil
}

func handleCalDAVGet(w http.ResponseWriter, r *http.Request, email string) {
	path := r.URL.Path

	// Extract UID from path
	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".ics") {
			uid = strings.TrimSuffix(p, ".ics")
			break
		}
	}

	var icalData, etag string
	err := db.QueryRow(`
		SELECT e.ical_data, e.etag FROM events e
		JOIN calendars c ON e.calendar_id = c.id
		JOIN virtual_users u ON c.user_id = u.id
		WHERE u.email = ? AND e.uid = ?`, email, uid).Scan(&icalData, &etag)

	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.Write([]byte(icalData))
}

func handleCalDAVDelete(w http.ResponseWriter, r *http.Request, email string) {
	path := r.URL.Path

	// Extract UID from path
	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".ics") {
			uid = strings.TrimSuffix(p, ".ics")
			break
		}
	}

	_, err := db.Exec(`
		DELETE e FROM events e
		JOIN calendars c ON e.calendar_id = c.id
		JOIN virtual_users u ON c.user_id = u.id
		WHERE u.email = ? AND e.uid = ?`, email, uid)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleMkCalendar(w http.ResponseWriter, r *http.Request, email string) {
	var userID int
	db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)

	// Extract calendar name from path
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	name := "New Calendar"
	if len(parts) > 2 {
		name = parts[len(parts)-1]
	}

	_, err := db.Exec("INSERT INTO calendars (user_id, name, ctag) VALUES (?, ?, ?)",
		userID, name, generateETag())

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func extractICalField(icalData, field string) string {
	// Normalize line endings
	icalData = strings.ReplaceAll(icalData, "\r\n", "\n")
	icalData = strings.ReplaceAll(icalData, "\r", "\n")

	// Find VEVENT section to avoid picking up VTIMEZONE fields
	veventStart := strings.Index(icalData, "BEGIN:VEVENT")
	veventEnd := strings.Index(icalData, "END:VEVENT")
	if veventStart >= 0 && veventEnd > veventStart {
		icalData = icalData[veventStart:veventEnd]
	}

	lines := strings.Split(icalData, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			return strings.TrimPrefix(line, field+":")
		}
		if strings.HasPrefix(line, field+";") {
			// Handle field;PARAM=value:actual_value format
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func getUserEmailFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
