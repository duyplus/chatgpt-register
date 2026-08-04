// Package codexreg Automates ChatGPT account registration using browser, exports accessToken directly,
// and outputs auth.json (accessToken structure). Called in batch by producer.
//
// Migrated from standalone got CLI tool:
//   - browser.go  : Opens chatgpt.com to complete registration (email -> code -> profile), extracts accessToken
//   - geoip.go    : Proxy parsing + timezone/coordinates/locale alignment by exit IP + resource blocking
//   - codex.go    : Assembles auth.json using accessToken
//
// Difference from CLI version: Verification code is no longer manually scanned via fmt.Scan,
// but automatically read from email via FetchCode callback provided by caller.
package codexreg

import (
	"context"
	"fmt"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

// Input Production parameters for a single account.
type Input struct {
	Email           string
	Password        string // Password used when registration requires password creation (auto-generated if empty)
	TwoFactorSecret string // Existing 2FA secret if already set
	FullName        string
	Age         string
	Proxy       string // Empty = direct connection
	Headless    bool
	IsLoginOnly bool   // True if logging into existing account for password & 2FA setup

	// FetchCode Fetches ChatGPT verification code sent to email. Implemented by producer via mailfetch.
	FetchCode func(ctx context.Context) (string, error)

	// Log Progress logging output (can be nil).
	Log func(format string, a ...any)

	// SaveShot Saves failure screenshot (PNG) for troubleshooting (can be nil).
	SaveShot func(png []byte)
}

// Result Production output result.
type Result struct {
	AccessToken     string         `json:"-"`
	AuthJSON        map[string]any `json:"auth_json"`  // Complete auth.json
	AccountID       string         `json:"account_id"` // Kept for caller compatibility
	UserID          string         `json:"user_id"`
	PlanType        string         `json:"plan_type"`
	TwoFactorSecret string         `json:"two_factor_secret"`
	Password        string         `json:"password,omitempty"`
}

func (in Input) logf(format string, a ...any) {
	if in.Log != nil {
		in.Log(format, a...)
	}
}

// Register Complete production of an account: Browser ChatGPT registration -> Get accessToken -> Assemble auth.json.
func Register(ctx context.Context, in Input) (*Result, error) {
	if in.FetchCode == nil {
		return nil, fmt.Errorf("missing FetchCode callback, cannot read verification code automatically")
	}
	if in.FullName == "" {
		in.FullName = genName()
	}
	if in.Age == "" {
		in.Age = genAge()
	}
	if in.Password == "" {
		in.Password = GenPassword(16)
	}

	accessToken, tfSecret, err := registerBrowser(ctx, in)
	if err != nil {
		if in.IsLoginOnly {
			return nil, fmt.Errorf("ChatGPT login & 2FA setup failed: %w", err)
		}
		return nil, fmt.Errorf("ChatGPT registration failed: %w", err)
	}

	return &Result{
		AccessToken:     accessToken,
		AuthJSON:        buildAuth(in, accessToken),
		TwoFactorSecret: tfSecret,
		Password:        in.Password,
	}, nil
}
