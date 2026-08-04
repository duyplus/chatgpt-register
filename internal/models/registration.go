package models

// Registration A ChatGPT + Codex account to be produced / produced.
//
// Status flow:
//
//	pending / registering / registered / register_failed
//
// After successful production, AuthData stores full auth.json (accessToken structure), exported during download.
// Shipped indicates whether it has been exported (downloading = exported).
type Registration struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Email     string `gorm:"size:255;not null;uniqueIndex" json:"email"`
	MailboxID uint   `gorm:"index" json:"mailbox_id"`
	Password  string `gorm:"size:255" json:"password"`

	Status  string `gorm:"size:32;default:pending" json:"status"`
	Shipped bool   `gorm:"default:false" json:"shipped"` // Shipped status: true=shipped

	// Production result
	AuthData        string `gorm:"type:text" json:"auth_data,omitempty"` // Full auth.json (contains access_token)
	AccountID       string `gorm:"size:255" json:"account_id"`
	UserID          string `gorm:"size:255" json:"user_id"`
	PlanType        string `gorm:"size:32" json:"plan_type"`
	TwoFactorSecret string `gorm:"type:text" json:"two_factor_secret"`
	IsMother        bool   `gorm:"default:false" json:"is_mother"` // Whether mother account (main account for mailbox)

	Log       string    `gorm:"type:text" json:"log,omitempty"` // Account execution log
	Shot      []byte    `gorm:"type:blob" json:"-"`             // Page screenshot on failure (PNG)
	Note      string    `gorm:"type:text" json:"note"`
	CreatedAt LocalTime `gorm:"autoCreateTime;type:text" json:"created_at"`
	UpdatedAt LocalTime `gorm:"autoUpdateTime;type:text" json:"updated_at"`
}
