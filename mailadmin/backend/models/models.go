package models

type Domain struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	WebmailEnabled bool   `json:"webmail_enabled"`
	CaldavEnabled  bool   `json:"caldav_enabled"`
	CarddavEnabled bool   `json:"carddav_enabled"`
}

type User struct {
	ID       int    `json:"id"`
	DomainID int    `json:"domain_id"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
	Domain   string `json:"domain,omitempty"`
	// Quota is the mailbox size limit in megabytes; 0 means unlimited.
	Quota       int    `json:"quota"`
	NotifyEmail string `json:"notify_email,omitempty"`
}

type Alias struct {
	ID          int    `json:"id"`
	DomainID    int    `json:"domain_id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Domain      string `json:"domain,omitempty"`
}

type SenderPermission struct {
	ID        int    `json:"id"`
	SendAs    string `json:"send_as"`
	LoginUser string `json:"login_user"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type PasswordChange struct {
	Password string `json:"password"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type MailingList struct {
	ListID      string `json:"list_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	MailHost    string `json:"mail_host"`
	MemberCount int    `json:"member_count"`
	Description string `json:"description"`
	Email       string `json:"email"`
}

type ListMember struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	MemberID    string `json:"member_id"`
}

type ListSettings struct {
	Description            string  `json:"description"`
	SubjectPrefix          string  `json:"subject_prefix"`
	ArchivePolicy          string  `json:"archive_policy"`
	DefaultMemberAction    string  `json:"default_member_action"`
	DefaultNonmemberAction string  `json:"default_nonmember_action"`
	DigestEnabled          bool    `json:"digest_enabled"`
	DigestFrequencyDays    float64 `json:"digest_frequency_days"`
	MaxMessageSize         int     `json:"max_message_size"`
	SubscriptionPolicy     string  `json:"subscription_policy"`
	UnsubscriptionPolicy   string  `json:"unsubscription_policy"`
	AdminImmedNotify       bool    `json:"admin_immed_notify"`
	AdminNotifyMchanges    bool    `json:"admin_notify_mchanges"`
	Advertised             bool    `json:"advertised"`
	AllowListPosts         bool    `json:"allow_list_posts"`
	ReplyGoesToList        bool    `json:"reply_goes_to_list"`
}
