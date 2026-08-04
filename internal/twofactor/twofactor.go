package twofactor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

// CleanSecret strips spaces, hyphens, and whitespace from a 2FA secret key and validates Base32 format.
func CleanSecret(secret string) string {
	secret = strings.ReplaceAll(secret, " ", "")
	secret = strings.ReplaceAll(secret, "-", "")
	secret = strings.TrimSpace(secret)
	secret = strings.ToUpper(secret)
	if len(secret) < 16 || len(secret) > 64 {
		return ""
	}
	invalidWords := []string{
		"SECURITYANDLOGIN", "SECURITYANDPRIVACY", "AUTHENTICATORAPP", "AUTHENTICATOR",
		"TROUBLESCANNING", "PROBLEMSCANNING", "SECURITY", "SETTINGS", "PASSWORD",
		"PARENTALCONTROLS", "PARENTALCONTROL", "PARENTAL",
		"DEVELOPERMODEELEVATEDRISK", "DEVELOPERMODE", "ELEVATEDRISK", "DEVELOPER", "ELEVATED", "RISK",
	}
	for _, w := range invalidWords {
		if strings.Contains(secret, w) {
			return ""
		}
	}
	for _, ch := range secret {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= '2' && ch <= '7')) {
			return ""
		}
	}
	return secret
}

// GetCode fetches current 6-digit 2FA code from https://2fa.live/tok/{secret}
func GetCode(ctx context.Context, secret string) (string, error) {
	clean := CleanSecret(secret)
	if clean == "" {
		return "", fmt.Errorf("2FA secret is empty")
	}

	url := "https://2fa.live/tok/" + clean
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to 2fa.live: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read 2fa.live response: %w", err)
	}

	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &res); err == nil && len(res.Token) == 6 {
		return res.Token, nil
	}

	if match := codeRe.FindStringSubmatch(string(body)); match != nil {
		return match[1], nil
	}

	return "", fmt.Errorf("invalid 2FA response from 2fa.live: %s", strings.TrimSpace(string(body)))
}
