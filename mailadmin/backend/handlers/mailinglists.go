package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"mailadmin/models"

	"github.com/gorilla/mux"
)

const (
	mailmanAPIURL  = "http://localhost:8001/3.1"
	mailmanAPIUser = "restadmin"
	mailmanAPIPass = "vC1p+QbeIY9hRxUvBpgqU4FZezTpsd7nIOLTxvskjWWt4P9M"
)

func mailmanRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, mailmanAPIURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(mailmanAPIUser, mailmanAPIPass)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return client.Do(req)
}

// GetMailingLists returns all mailing lists
func GetMailingLists(w http.ResponseWriter, r *http.Request) {
	resp, err := mailmanRequest("GET", "/lists", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Entries []struct {
			ListID      string `json:"list_id"`
			ListName    string `json:"list_name"`
			DisplayName string `json:"display_name"`
			MailHost    string `json:"mail_host"`
			MemberCount int    `json:"member_count"`
			Description string `json:"description"`
			FQDNListID  string `json:"fqdn_listname"`
		} `json:"entries"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	lists := make([]models.MailingList, 0)
	for _, e := range result.Entries {
		lists = append(lists, models.MailingList{
			ListID:      e.ListID,
			Name:        e.ListName,
			DisplayName: e.DisplayName,
			MailHost:    e.MailHost,
			MemberCount: e.MemberCount,
			Description: e.Description,
			Email:       e.FQDNListID,
		})
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: lists})
}

// CreateMailingList creates a new mailing list
func CreateMailingList(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Domain      string `json:"domain"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	// First check if domain exists in Mailman
	domainResp, err := mailmanRequest("GET", "/domains/"+req.Domain, nil)
	if err != nil || domainResp.StatusCode == 404 {
		// Create domain first
		data := url.Values{}
		data.Set("mail_host", req.Domain)
		mailmanRequest("POST", "/domains", strings.NewReader(data.Encode()))
	}
	if domainResp != nil {
		domainResp.Body.Close()
	}

	// Create the list
	fqdnListname := fmt.Sprintf("%s@%s", req.Name, req.Domain)
	data := url.Values{}
	data.Set("fqdn_listname", fqdnListname)

	resp, err := mailmanRequest("POST", "/lists", strings.NewReader(data.Encode()))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: string(bodyBytes)})
		return
	}

	// Update description if provided
	if req.Description != "" {
		listID := fmt.Sprintf("%s.%s", req.Name, req.Domain)
		data := url.Values{}
		data.Set("description", req.Description)
		mailmanRequest("PATCH", "/lists/"+listID+"/config", strings.NewReader(data.Encode()))
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Message: "List created"})
}

// DeleteMailingList deletes a mailing list
func DeleteMailingList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["id"]

	resp, err := mailmanRequest("DELETE", "/lists/"+listID, nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Failed to delete list"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "List deleted"})
}

// GetMailingListMembers returns members of a mailing list
func GetMailingListMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["id"]

	resp, err := mailmanRequest("GET", "/lists/"+listID+"/roster/member", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Entries []struct {
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			Role        string `json:"role"`
			MemberID    string `json:"member_id"`
		} `json:"entries"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	members := make([]models.ListMember, 0)
	for _, e := range result.Entries {
		members = append(members, models.ListMember{
			Email:       e.Email,
			DisplayName: e.DisplayName,
			Role:        e.Role,
			MemberID:    e.MemberID,
		})
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: members})
}

// AddMailingListMember adds a member to a mailing list
func AddMailingListMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["id"]

	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	data := url.Values{}
	data.Set("list_id", listID)
	data.Set("subscriber", req.Email)
	data.Set("pre_verified", "true")
	data.Set("pre_confirmed", "true")
	data.Set("pre_approved", "true")
	if req.DisplayName != "" {
		data.Set("display_name", req.DisplayName)
	}

	resp, err := mailmanRequest("POST", "/members", strings.NewReader(data.Encode()))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: string(bodyBytes)})
		return
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Message: "Member added"})
}

// RemoveMailingListMember removes a member from a mailing list
func RemoveMailingListMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["id"]
	email := vars["email"]

	// First get the member ID
	resp, err := mailmanRequest("GET", "/lists/"+listID+"/roster/member", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	var result struct {
		Entries []struct {
			Email    string `json:"email"`
			MemberID string `json:"member_id"`
			SelfLink string `json:"self_link"`
		} `json:"entries"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	var memberURL string
	for _, e := range result.Entries {
		if e.Email == email {
			memberURL = e.SelfLink
			break
		}
	}

	if memberURL == "" {
		respondJSON(w, http.StatusNotFound, models.APIResponse{Success: false, Message: "Member not found"})
		return
	}

	// Extract the member endpoint from self_link
	parts := strings.Split(memberURL, "/3.1")
	if len(parts) < 2 {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: "Invalid member URL"})
		return
	}

	resp, err = mailmanRequest("DELETE", parts[1], nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Failed to remove member"})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Member removed"})
}

// GetMailingListSettings returns settings for a mailing list
func GetMailingListSettings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["id"]

	resp, err := mailmanRequest("GET", "/lists/"+listID+"/config", nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}

	// Extract relevant settings
	settings := models.ListSettings{
		Description:            getString(config, "description"),
		SubjectPrefix:          getString(config, "subject_prefix"),
		ArchivePolicy:          getString(config, "archive_policy"),
		DefaultMemberAction:    getString(config, "default_member_action"),
		DefaultNonmemberAction: getString(config, "default_nonmember_action"),
		DigestEnabled:          getBool(config, "digest_send_periodic"),
		DigestFrequencyDays:    getFloat(config, "digest_volume_frequency"),
		MaxMessageSize:         getInt(config, "max_message_size"),
		SubscriptionPolicy:     getString(config, "subscription_policy"),
		UnsubscriptionPolicy:   getString(config, "unsubscription_policy"),
		AdminImmedNotify:       getBool(config, "admin_immed_notify"),
		AdminNotifyMchanges:    getBool(config, "admin_notify_mchanges"),
		Advertised:             getBool(config, "advertised"),
		AllowListPosts:         getBool(config, "allow_list_posts"),
		ReplyGoesToList:        getBool(config, "reply_goes_to_list"),
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: settings})
}

// UpdateMailingListSettings updates settings for a mailing list
func UpdateMailingListSettings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	listID := vars["id"]

	var settings models.ListSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid request"})
		return
	}

	data := url.Values{}
	if settings.Description != "" {
		data.Set("description", settings.Description)
	}
	if settings.SubjectPrefix != "" {
		data.Set("subject_prefix", settings.SubjectPrefix)
	}
	if settings.ArchivePolicy != "" {
		data.Set("archive_policy", settings.ArchivePolicy)
	}
	if settings.DefaultMemberAction != "" {
		data.Set("default_member_action", settings.DefaultMemberAction)
	}
	if settings.DefaultNonmemberAction != "" {
		data.Set("default_nonmember_action", settings.DefaultNonmemberAction)
	}
	if settings.SubscriptionPolicy != "" {
		data.Set("subscription_policy", settings.SubscriptionPolicy)
	}
	if settings.UnsubscriptionPolicy != "" {
		data.Set("unsubscription_policy", settings.UnsubscriptionPolicy)
	}

	data.Set("digest_send_periodic", fmt.Sprintf("%t", settings.DigestEnabled))
	data.Set("admin_immed_notify", fmt.Sprintf("%t", settings.AdminImmedNotify))
	data.Set("admin_notify_mchanges", fmt.Sprintf("%t", settings.AdminNotifyMchanges))
	data.Set("advertised", fmt.Sprintf("%t", settings.Advertised))
	data.Set("allow_list_posts", fmt.Sprintf("%t", settings.AllowListPosts))

	if settings.MaxMessageSize > 0 {
		data.Set("max_message_size", fmt.Sprintf("%d", settings.MaxMessageSize))
	}
	if settings.DigestFrequencyDays > 0 {
		data.Set("digest_volume_frequency", fmt.Sprintf("%.1f", settings.DigestFrequencyDays))
	}

	// reply_goes_to_list is an enum in Mailman
	if settings.ReplyGoesToList {
		data.Set("reply_goes_to_list", "explicit_header")
	} else {
		data.Set("reply_goes_to_list", "no_munging")
	}

	resp, err := mailmanRequest("PATCH", "/lists/"+listID+"/config", strings.NewReader(data.Encode()))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, models.APIResponse{Success: false, Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		respondJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: string(bodyBytes)})
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Settings updated"})
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 0
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}
