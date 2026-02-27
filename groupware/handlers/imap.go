package handlers

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/gorilla/mux"
)

type Folder struct {
	Name       string   `json:"name"`
	Delimiter  string   `json:"delimiter"`
	Unread     uint32   `json:"unread"`
	Total      uint32   `json:"total"`
	Attributes []string `json:"attributes,omitempty"`
}

type MessageSummary struct {
	UID           uint32    `json:"uid"`
	Subject       string    `json:"subject"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	Date          time.Time `json:"date"`
	Size          uint32    `json:"size"`
	Seen          bool      `json:"seen"`
	Flagged       bool      `json:"flagged"`
	HasAttachment bool      `json:"has_attachment"`
}

type MessageDetail struct {
	UID         uint32            `json:"uid"`
	Subject     string            `json:"subject"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Cc          string            `json:"cc"`
	Date        time.Time         `json:"date"`
	TextBody    string            `json:"text_body"`
	HTMLBody    string            `json:"html_body"`
	Attachments []AttachmentInfo  `json:"attachments"`
	Seen        bool              `json:"seen"`
	Flagged     bool              `json:"flagged"`
}

type AttachmentInfo struct {
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

func getIMAPClient(r *http.Request) (*client.Client, error) {
	claims := GetUserFromContext(r)
	if claims == nil {
		return nil, nil
	}

	c, err := client.DialTLS("localhost:993", imapTLSConfig)
	if err != nil {
		return nil, err
	}

	if err := c.Login(claims.Email, claims.Password); err != nil {
		c.Logout()
		return nil, err
	}

	return c, nil
}

func GetFolders(w http.ResponseWriter, r *http.Request) {
	c, err := getIMAPClient(r)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "IMAP connection failed",
		})
		return
	}
	defer c.Logout()

	mailboxes := make(chan *imap.MailboxInfo, 100)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	var folders []Folder
	for mb := range mailboxes {
		folder := Folder{
			Name:       mb.Name,
			Delimiter:  mb.Delimiter,
			Attributes: mb.Attributes,
		}

		status, err := c.Status(mb.Name, []imap.StatusItem{imap.StatusMessages, imap.StatusUnseen})
		if err == nil {
			folder.Unread = status.Unseen
			folder.Total = status.Messages
		}

		folders = append(folders, folder)
	}

	if err := <-done; err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to list folders",
		})
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    folders,
	})
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	folder := vars["folder"]

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	c, err := getIMAPClient(r)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "IMAP connection failed",
		})
		return
	}
	defer c.Logout()

	mbox, err := c.Select(folder, false)
	if err != nil {
		// Folder might be unselectable (e.g., \Noselect namespace container)
		// Return empty list instead of error
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"messages": []MessageSummary{},
				"total":    0,
				"page":     page,
				"pages":    0,
			},
		})
		return
	}

	if mbox.Messages == 0 {
		RespondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"messages": []MessageSummary{},
				"total":    0,
				"page":     page,
				"pages":    0,
			},
		})
		return
	}

	total := mbox.Messages
	from := uint32(1)
	to := total

	if int(total) > perPage*page {
		from = total - uint32(perPage*page) + 1
	}
	if int(total) > perPage*(page-1) {
		to = total - uint32(perPage*(page-1))
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(from, to)

	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchRFC822Size, imap.FetchUid, imap.FetchBodyStructure}

	messages := make(chan *imap.Message, 100)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqSet, items, messages)
	}()

	var msgList []MessageSummary
	for msg := range messages {
		if msg.Envelope == nil {
			continue
		}

		summary := MessageSummary{
			UID:     msg.Uid,
			Subject: decodeHeader(msg.Envelope.Subject),
			Date:    msg.Envelope.Date,
			Size:    msg.Size,
		}

		if len(msg.Envelope.From) > 0 {
			summary.From = formatAddress(msg.Envelope.From[0])
		}
		if len(msg.Envelope.To) > 0 {
			summary.To = formatAddress(msg.Envelope.To[0])
		}

		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				summary.Seen = true
			}
			if flag == imap.FlaggedFlag {
				summary.Flagged = true
			}
		}

		if msg.BodyStructure != nil {
			summary.HasAttachment = hasAttachments(msg.BodyStructure)
		}

		msgList = append(msgList, summary)
	}

	if err := <-done; err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to fetch messages",
		})
		return
	}

	// Reverse to show newest first
	for i, j := 0, len(msgList)-1; i < j; i, j = i+1, j-1 {
		msgList[i], msgList[j] = msgList[j], msgList[i]
	}

	pages := int(total) / perPage
	if int(total)%perPage > 0 {
		pages++
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"messages": msgList,
			"total":    total,
			"page":     page,
			"pages":    pages,
		},
	})
}

func GetMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	folder := vars["folder"]
	uid, _ := strconv.ParseUint(vars["uid"], 10, 32)

	c, err := getIMAPClient(r)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "IMAP connection failed",
		})
		return
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to select folder",
		})
		return
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uint32(uid))

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, section.FetchItem()}

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqSet, items, messages)
	}()

	msg := <-messages
	if err := <-done; err != nil || msg == nil {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"message": "Message not found",
		})
		return
	}

	detail := MessageDetail{
		UID:     msg.Uid,
		Subject: decodeHeader(msg.Envelope.Subject),
		Date:    msg.Envelope.Date,
	}

	if len(msg.Envelope.From) > 0 {
		detail.From = formatAddress(msg.Envelope.From[0])
	}
	if len(msg.Envelope.To) > 0 {
		var toAddrs []string
		for _, addr := range msg.Envelope.To {
			toAddrs = append(toAddrs, formatAddress(addr))
		}
		detail.To = strings.Join(toAddrs, ", ")
	}
	if len(msg.Envelope.Cc) > 0 {
		var ccAddrs []string
		for _, addr := range msg.Envelope.Cc {
			ccAddrs = append(ccAddrs, formatAddress(addr))
		}
		detail.Cc = strings.Join(ccAddrs, ", ")
	}

	for _, flag := range msg.Flags {
		if flag == imap.SeenFlag {
			detail.Seen = true
		}
		if flag == imap.FlaggedFlag {
			detail.Flagged = true
		}
	}

	bodySection := msg.GetBody(section)
	if bodySection != nil {
		mr, err := mail.CreateReader(bodySection)
		if err == nil {
			attachmentIndex := 0
			for {
				p, err := mr.NextPart()
				if err != nil {
					break
				}

				switch h := p.Header.(type) {
				case *mail.InlineHeader:
					ct, _, _ := h.ContentType()
					body, _ := io.ReadAll(p.Body)

					if strings.HasPrefix(ct, "text/plain") && detail.TextBody == "" {
						detail.TextBody = string(body)
					} else if strings.HasPrefix(ct, "text/html") {
						detail.HTMLBody = string(body)
					}

				case *mail.AttachmentHeader:
					filename, _ := h.Filename()
					ct, _, _ := h.ContentType()
					body, _ := io.ReadAll(p.Body)

					detail.Attachments = append(detail.Attachments, AttachmentInfo{
						Index:       attachmentIndex,
						Filename:    filename,
						ContentType: ct,
						Size:        len(body),
					})
					attachmentIndex++
				}
			}
		}
	}

	// Mark as seen
	c.UidStore(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.SeenFlag}, nil)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    detail,
	})
}

func DeleteMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	folder := vars["folder"]
	uid, _ := strconv.ParseUint(vars["uid"], 10, 32)

	c, err := getIMAPClient(r)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "IMAP connection failed",
		})
		return
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to select folder",
		})
		return
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uint32(uid))

	err = c.UidStore(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to delete message",
		})
		return
	}

	c.Expunge(nil)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Message deleted",
	})
}

func MoveMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	folder := vars["folder"]
	uid, _ := strconv.ParseUint(vars["uid"], 10, 32)

	var req struct {
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	c, err := getIMAPClient(r)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "IMAP connection failed",
		})
		return
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to select folder",
		})
		return
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uint32(uid))

	err = c.UidCopy(seqSet, req.Destination)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to move message",
		})
		return
	}

	c.UidStore(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
	c.Expunge(nil)

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Message moved",
	})
}

func UpdateFlags(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	folder := vars["folder"]
	uid, _ := strconv.ParseUint(vars["uid"], 10, 32)

	var req struct {
		Seen    *bool `json:"seen,omitempty"`
		Flagged *bool `json:"flagged,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	c, err := getIMAPClient(r)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "IMAP connection failed",
		})
		return
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		RespondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to select folder",
		})
		return
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uint32(uid))

	if req.Seen != nil {
		if *req.Seen {
			c.UidStore(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.SeenFlag}, nil)
		} else {
			c.UidStore(seqSet, imap.FormatFlagsOp(imap.RemoveFlags, true), []interface{}{imap.SeenFlag}, nil)
		}
	}

	if req.Flagged != nil {
		if *req.Flagged {
			c.UidStore(seqSet, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.FlaggedFlag}, nil)
		} else {
			c.UidStore(seqSet, imap.FormatFlagsOp(imap.RemoveFlags, true), []interface{}{imap.FlaggedFlag}, nil)
		}
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Flags updated",
	})
}

func GetAttachment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	folder := vars["folder"]
	uid, _ := strconv.ParseUint(vars["uid"], 10, 32)
	index, _ := strconv.Atoi(vars["index"])

	c, err := getIMAPClient(r)
	if err != nil {
		http.Error(w, "IMAP connection failed", http.StatusInternalServerError)
		return
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		http.Error(w, "Failed to select folder", http.StatusInternalServerError)
		return
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uint32(uid))

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
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	bodySection := msg.GetBody(section)
	if bodySection == nil {
		http.Error(w, "Body not found", http.StatusNotFound)
		return
	}

	mr, err := mail.CreateReader(bodySection)
	if err != nil {
		http.Error(w, "Failed to parse message", http.StatusInternalServerError)
		return
	}

	attachmentIndex := 0
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}

		if h, ok := p.Header.(*mail.AttachmentHeader); ok {
			if attachmentIndex == index {
				filename, _ := h.Filename()
				ct, _, _ := h.ContentType()
				body, _ := io.ReadAll(p.Body)

				w.Header().Set("Content-Type", ct)
				w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
				w.Write(body)
				return
			}
			attachmentIndex++
		}
	}

	http.Error(w, "Attachment not found", http.StatusNotFound)
}

// Helper functions
func formatAddress(addr *imap.Address) string {
	if addr == nil {
		return ""
	}
	name := decodeHeader(addr.PersonalName)
	email := addr.MailboxName + "@" + addr.HostName
	if name != "" {
		return name + " <" + email + ">"
	}
	return email
}

func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

func hasAttachments(bs *imap.BodyStructure) bool {
	if bs == nil {
		return false
	}
	if bs.Disposition == "attachment" {
		return true
	}
	for _, part := range bs.Parts {
		if hasAttachments(part) {
			return true
		}
	}
	return false
}
