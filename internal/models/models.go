package models

import "time"

// Mailbox Mailbox management
//
// Status flow: unverified / verifying / verify_failed / verified
type Mailbox struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Email           string    `gorm:"size:255;not null;uniqueIndex" json:"email"`
	Password        string    `gorm:"size:255" json:"password"`
	ClientID        string    `gorm:"size:255" json:"client_id"`
	RefreshToken    string    `gorm:"type:text" json:"refresh_token"`
	Provider        string    `gorm:"size:64" json:"provider"` // gmail / outlook / temp mail...
	PurchaseID      int       `gorm:"default:0" json:"purchase_id"` // vary.email purchase ID (valid when provider=varymail)
	Status          string    `gorm:"size:32;default:unverified" json:"status"`
	Note            string    `gorm:"type:text" json:"note"`
	RegisterCount   int       `gorm:"-" json:"register_count"`
	RegisterLimit   int       `gorm:"-" json:"register_limit"`
	CreatedAt       LocalTime `gorm:"autoCreateTime;type:text" json:"created_at"`
	UpdatedAt       LocalTime `gorm:"autoUpdateTime;type:text" json:"updated_at"`
}

// Setting System settings (key-value)
type Setting struct {
	Key       string    `gorm:"primaryKey;size:128" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt LocalTime `gorm:"autoUpdateTime;type:text" json:"updated_at"`
}

// Admin Admin user account.
//
// Token stores current unique valid JWT.
// TokenIssuedAt stores token issue time, used for "auto renewal after 2 hours".
type Admin struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Username      string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash  string    `gorm:"size:255;not null" json:"-"`
	Token         string    `gorm:"type:text" json:"-"`
	TokenIssuedAt time.Time `json:"-"`
	CreatedAt     LocalTime `gorm:"autoCreateTime;type:text" json:"created_at"`
	UpdatedAt     LocalTime `gorm:"autoUpdateTime;type:text" json:"updated_at"`
}
