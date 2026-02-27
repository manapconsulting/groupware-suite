package handlers

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

type SendRequest struct {
	To                 []string            `json:"to"`
	Cc                 []string            `json:"cc,omitempty"`
	Bcc                []string            `json:"bcc,omitempty"`
	Subject            string              `json:"subject"`
	Body               string              `json:"body"`
	IsHTML             bool                `json:"is_html"`
	Attachments        []AttachData        `json:"attachments,omitempty"`
	ForwardAttachments *ForwardAttachments `json:"forward_attachments,omitempty"`
}

type AttachData struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        string `json:"data"` // base64 encoded
}

type ForwardAttachments struct {
	SourceFolder string `json:"source_folder"`
	SourceUID    uint32 `json:"source_uid"`
	Indexes      []int  `json:"indexes"`
}

func SendMessage(w http.ResponseWriter, r *http.Request) {
	claims := GetUserFromContext(r)
	if claims == nil {
		RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	if len(req.To) == 0 {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "At least one recipient required",
		})
		return
	}

	// Fetch forward attachments if specified
	if req.ForwardAttachments != nil && len(req.ForwardAttachments.Indexes) > 0 {
		forwardedAtts, err := fetchAttachmentsFromMessage(
			claims.Email,
			claims.Password,
			req.ForwardAttachments.SourceFolder,
			req.ForwardAttachments.SourceUID,
			req.ForwardAttachments.Indexes,
		)
		if err == nil {
			req.Attachments = append(req.Attachments, forwardedAtts...)
		}
	}

	// Build email
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Headers
	headers := make(textproto.MIMEHeader)
	headers.Set("From", claims.Email)
	headers.Set("To", strings.Join(req.To, ", "))
	if len(req.Cc) > 0 {
		headers.Set("Cc", strings.Join(req.Cc, ", "))
	}
	headers.Set("Subject", encodeSubject(req.Subject))
	headers.Set("Date", time.Now().Format(time.RFC1123Z))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", "multipart/mixed; boundary="+writer.Boundary())

	// Write headers
	var headerBuf bytes.Buffer
	for k, v := range headers {
		headerBuf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v[0]))
	}
	headerBuf.WriteString("\r\n")

	// Body part
	contentType := "text/plain; charset=UTF-8"
	if req.IsHTML {
		contentType = "text/html; charset=UTF-8"
	}

	bodyPart, _ := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {contentType},
		"Content-Transfer-Encoding": {"base64"},
	})
	bodyPart.Write([]byte(base64.StdEncoding.EncodeToString([]byte(req.Body))))

	// Attachments
	for _, att := range req.Attachments {
		attPart, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {att.ContentType + "; name=\"" + att.Filename + "\""},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {"attachment; filename=\"" + att.Filename + "\""},
		})
		attPart.Write([]byte(att.Data))
	}

	writer.Close()

	// Combine headers and body
	var message bytes.Buffer
	message.Write(headerBuf.Bytes())
	message.Write(buf.Bytes())

	// All recipients
	allRecipients := append(append(req.To, req.Cc...), req.Bcc...)

	// Send via SMTP with authentication
	err := sendSMTP(claims.Email, claims.Password, allRecipients, message.Bytes())
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to send email: " + err.Error(),
		})
		return
	}

	// Save to Sent folder via IMAP
	go saveToSent(claims.Email, claims.Password, message.Bytes())

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Email sent successfully",
	})
}

func sendSMTP(from, password string, to []string, msg []byte) error {
	conn, err := smtp.Dial("localhost:587")
	if err != nil {
		return fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "localhost",
	}
	if err := conn.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starttls failed: %v", err)
	}

	auth := smtp.PlainAuth("", from, password, "localhost")
	if err := conn.Auth(auth); err != nil {
		return fmt.Errorf("auth failed: %v", err)
	}

	if err := conn.Mail(from); err != nil {
		return fmt.Errorf("mail from failed: %v", err)
	}

	for _, addr := range to {
		if err := conn.Rcpt(addr); err != nil {
			return fmt.Errorf("rcpt to failed: %v", err)
		}
	}

	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("data failed: %v", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("close failed: %v", err)
	}

	return conn.Quit()
}

func saveToSent(email, password string, msg []byte) {
	c, err := getIMAPClientDirect(email, password)
	if err != nil {
		return
	}
	defer c.Logout()

	sentFolders := []string{"Sent", "INBOX.Sent", "Sent Items", "Sent Messages"}
	var sentFolder string

	for _, folder := range sentFolders {
		_, err := c.Select(folder, false)
		if err == nil {
			sentFolder = folder
			break
		}
	}

	if sentFolder == "" {
		c.Create("Sent")
		sentFolder = "Sent"
	}

	c.Append(sentFolder, nil, time.Now(), bytes.NewReader(msg))
}

func getIMAPClientDirect(email, password string) (*client.Client, error) {
	c, err := client.DialTLS("localhost:993", imapTLSConfig)
	if err != nil {
		return nil, err
	}

	if err := c.Login(email, password); err != nil {
		c.Logout()
		return nil, err
	}

	return c, nil
}

func fetchAttachmentsFromMessage(email, password, folder string, uid uint32, indexes []int) ([]AttachData, error) {
	c, err := getIMAPClientDirect(email, password)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		return nil, err
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	msg := <-messages
	<-done

	if msg == nil {
		return nil, fmt.Errorf("message not found")
	}

	bodySection := msg.GetBody(section)
	if bodySection == nil {
		return nil, fmt.Errorf("body not found")
	}

	mr, err := mail.CreateReader(bodySection)
	if err != nil {
		return nil, err
	}

	var attachments []AttachData
	attIndex := 0

	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}

		switch part.Header.(type) {
		case *mail.AttachmentHeader:
			for _, idx := range indexes {
				if idx == attIndex {
					h := part.Header.(*mail.AttachmentHeader)
					filename, _ := h.Filename()
					contentType, _, _ := h.ContentType()

					data, err := io.ReadAll(part.Body)
					if err != nil {
						continue
					}

					attachments = append(attachments, AttachData{
						Filename:    filename,
						ContentType: contentType,
						Data:        base64.StdEncoding.EncodeToString(data),
					})
					break
				}
			}
			attIndex++
		}
	}

	return attachments, nil
}

func encodeSubject(s string) string {
	needsEncoding := false
	for _, r := range s {
		if r > 127 {
			needsEncoding = true
			break
		}
	}

	if !needsEncoding {
		return s
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	return "=?UTF-8?B?" + encoded + "?="
}
