package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func WellKnownCardDAV(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/carddav/", http.StatusMovedPermanently)
}

func CardDAVHandler(w http.ResponseWriter, r *http.Request) {
	email := r.Header.Get("X-User-Email")
	if email == "" {
		email = getUserEmailFromPath(r.URL.Path)
	}

	switch r.Method {
	case "OPTIONS":
		handleCardDAVOptions(w, r)
	case "PROPFIND":
		handleCardDAVPropfind(w, r, email)
	case "REPORT":
		handleCardDAVReport(w, r, email)
	case "PUT":
		handleCardDAVPut(w, r, email)
	case "GET":
		handleCardDAVGet(w, r, email)
	case "DELETE":
		handleCardDAVDelete(w, r, email)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleCardDAVOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "OPTIONS, GET, PUT, DELETE, PROPFIND, REPORT")
	w.Header().Set("DAV", "1, 2, 3, addressbook")
	w.WriteHeader(http.StatusOK)
}

func handleCardDAVPropfind(w http.ResponseWriter, r *http.Request, email string) {
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
	w.WriteHeader(207)

	if path == "/carddav/" || path == "/carddav" {
		writeCardDAVPrincipal(w, email)
	} else if strings.Contains(path, ".vcf") {
		writeContactPropfind(w, email, path)
	} else if addressbookID := getAddressbookIDFromPath(path, email); addressbookID > 0 {
		writeAddressBookCollection(w, email, userID, addressbookID, depth)
	} else {
		writeAddressBookHome(w, email, userID, depth)
	}
}

func getAddressbookIDFromPath(path string, email string) int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Path like /carddav/user@email/addressbook/ or /carddav/user@email/addressbook2/
	if len(parts) >= 3 {
		abPath := parts[2]

		// Handle "addressbook", "addressbook2", etc.
		if strings.HasPrefix(abPath, "addressbook") {
			var userID int
			db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)
			if userID > 0 {
				// Parse the number suffix (default to 1 for "addressbook")
				offset := 0
				if abPath != "addressbook" {
					fmt.Sscanf(abPath, "addressbook%d", &offset)
					offset-- // Convert to 0-based offset
				}

				// Get the nth addressbook
				var id int
				db.QueryRow("SELECT id FROM addressbooks WHERE user_id = ? ORDER BY id LIMIT 1 OFFSET ?", userID, offset).Scan(&id)
				return id
			}
		}

		// Handle numeric ID (legacy support)
		var id int
		fmt.Sscanf(abPath, "%d", &id)
		return id
	}
	return 0
}

func writeAddressBookCollection(w http.ResponseWriter, email string, userID int, addressbookID int, depth string) {
	var name, ctag string
	err := db.QueryRow("SELECT name, ctag FROM addressbooks WHERE id = ? AND user_id = ?", addressbookID, userID).Scan(&name, &ctag)
	if err != nil {
		http.Error(w, "Addressbook not found", http.StatusNotFound)
		return
	}

	var responses strings.Builder

	// Add the addressbook itself - use "addressbook" path for consistency
	responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/carddav/%s/addressbook/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:addressbook/>
        </D:resourcetype>
        <D:displayname>%s</D:displayname>
        <CS:getctag xmlns:CS="http://calendarserver.org/ns/">%s</CS:getctag>
        <C:supported-address-data>
          <C:address-data-type content-type="text/vcard" version="3.0"/>
          <C:address-data-type content-type="text/vcard" version="4.0"/>
        </C:supported-address-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, name, ctag))

	// If depth is not 0, list contacts
	if depth != "0" {
		rows, _ := db.Query("SELECT uid, etag FROM contacts WHERE addressbook_id = ?", addressbookID)
		defer rows.Close()

		for rows.Next() {
			var uid, etag string
			rows.Scan(&uid, &etag)

			responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/carddav/%s/addressbook/%s.vcf</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"%s"</D:getetag>
        <D:getcontenttype>text/vcard; charset=utf-8</D:getcontenttype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, uid, etag))
		}
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func writeCardDAVPrincipal(w http.ResponseWriter, email string) {
	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:response>
    <D:href>/carddav/</D:href>
    <D:propstat>
      <D:prop>
        <D:current-user-principal>
          <D:href>/carddav/%s/</D:href>
        </D:current-user-principal>
        <D:principal-URL>
          <D:href>/carddav/%s/</D:href>
        </D:principal-URL>
        <D:resourcetype>
          <D:collection/>
        </D:resourcetype>
        <D:displayname>%s</D:displayname>
        <C:addressbook-home-set>
          <D:href>/carddav/%s/</D:href>
        </C:addressbook-home-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, email, email, email, email)
	w.Write([]byte(response))
}

func writeAddressBookHome(w http.ResponseWriter, email string, userID int, depth string) {
	var responses strings.Builder

	responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/carddav/%s/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
        <D:displayname>%s</D:displayname>
        <D:current-user-principal>
          <D:href>/carddav/%s/</D:href>
        </D:current-user-principal>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, email, email))

	if depth != "0" {
		rows, _ := db.Query("SELECT id, name, ctag FROM addressbooks WHERE user_id = ? ORDER BY id LIMIT 10", userID)
		defer rows.Close()

		abNum := 0
		for rows.Next() {
			var id int
			var name, ctag string
			rows.Scan(&id, &name, &ctag)

			// Use "addressbook" path for the first, "addressbook2", "addressbook3" etc for others
			abPath := "addressbook"
			if abNum > 0 {
				abPath = fmt.Sprintf("addressbook%d", abNum+1)
			}
			abNum++

			responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/carddav/%s/%s/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:addressbook/>
        </D:resourcetype>
        <D:displayname>%s</D:displayname>
        <CS:getctag xmlns:CS="http://calendarserver.org/ns/">%s</CS:getctag>
        <C:supported-address-data>
          <C:address-data-type content-type="text/vcard" version="3.0"/>
          <C:address-data-type content-type="text/vcard" version="4.0"/>
        </C:supported-address-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, abPath, name, ctag))
		}
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func writeContactPropfind(w http.ResponseWriter, email string, path string) {
	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".vcf") {
			uid = strings.TrimSuffix(p, ".vcf")
			break
		}
	}

	var etag string
	err := db.QueryRow(`
		SELECT c.etag FROM contacts c
		JOIN addressbooks a ON c.addressbook_id = a.id
		JOIN virtual_users u ON a.user_id = u.id
		WHERE u.email = ? AND c.uid = ?`, email, uid).Scan(&etag)

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
        <D:getcontenttype>text/vcard; charset=utf-8</D:getcontenttype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, path, etag)
	w.Write([]byte(response))
}

func handleCardDAVReport(w http.ResponseWriter, r *http.Request, email string) {
	var userID int
	db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207)

	rows, _ := db.Query(`
		SELECT c.uid, c.etag, c.vcard_data
		FROM contacts c
		JOIN addressbooks a ON c.addressbook_id = a.id
		WHERE a.user_id = ?`, userID)
	defer rows.Close()

	var responses strings.Builder
	for rows.Next() {
		var uid, etag, vcardData string
		rows.Scan(&uid, &etag, &vcardData)

		responses.WriteString(fmt.Sprintf(`
  <D:response>
    <D:href>/carddav/%s/contacts/%s.vcf</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"%s"</D:getetag>
        <C:address-data>%s</C:address-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`, email, uid, etag, xmlEscape(vcardData)))
	}

	response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">%s
</D:multistatus>`, responses.String())
	w.Write([]byte(response))
}

func handleCardDAVPut(w http.ResponseWriter, r *http.Request, email string) {
	var userID int
	db.QueryRow("SELECT id FROM virtual_users WHERE email = ?", email).Scan(&userID)

	var addressbookID int
	db.QueryRow("SELECT id FROM addressbooks WHERE user_id = ? LIMIT 1", userID).Scan(&addressbookID)

	path := r.URL.Path
	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".vcf") {
			uid = strings.TrimSuffix(p, ".vcf")
			break
		}
	}

	body, _ := io.ReadAll(r.Body)
	vcardData := string(body)

	fullName := extractVCardField(vcardData, "FN")
	emailAddr := extractVCardField(vcardData, "EMAIL")
	phone := extractVCardField(vcardData, "TEL")

	etag := generateETag()

	_, err := db.Exec(`
		INSERT INTO contacts (addressbook_id, uid, full_name, email, phone, vcard_data, etag)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		full_name = VALUES(full_name),
		email = VALUES(email),
		phone = VALUES(phone),
		vcard_data = VALUES(vcard_data),
		etag = VALUES(etag)`,
		addressbookID, uid, fullName, emailAddr, phone, vcardData, etag)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db.Exec("UPDATE addressbooks SET ctag = ? WHERE id = ?", generateETag(), addressbookID)

	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.WriteHeader(http.StatusCreated)
}

func handleCardDAVGet(w http.ResponseWriter, r *http.Request, email string) {
	path := r.URL.Path

	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".vcf") {
			uid = strings.TrimSuffix(p, ".vcf")
			break
		}
	}

	var vcardData, etag string
	err := db.QueryRow(`
		SELECT c.vcard_data, c.etag FROM contacts c
		JOIN addressbooks a ON c.addressbook_id = a.id
		JOIN virtual_users u ON a.user_id = u.id
		WHERE u.email = ? AND c.uid = ?`, email, uid).Scan(&vcardData, &etag)

	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.Write([]byte(vcardData))
}

func handleCardDAVDelete(w http.ResponseWriter, r *http.Request, email string) {
	path := r.URL.Path

	parts := strings.Split(path, "/")
	var uid string
	for _, p := range parts {
		if strings.HasSuffix(p, ".vcf") {
			uid = strings.TrimSuffix(p, ".vcf")
			break
		}
	}

	_, err := db.Exec(`
		DELETE c FROM contacts c
		JOIN addressbooks a ON c.addressbook_id = a.id
		JOIN virtual_users u ON a.user_id = u.id
		WHERE u.email = ? AND c.uid = ?`, email, uid)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func extractVCardField(vcardData, field string) string {
	lines := strings.Split(vcardData, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			return strings.TrimPrefix(line, field+":")
		}
		if strings.HasPrefix(line, field+";") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}
	return ""
}
