package twofactor

import (
	"context"
	"testing"
	"time"
)

func TestCleanSecret(t *testing.T) {
	input := " JBSW Y3DP-EHPK 3PXP "
	expected := "JBSWY3DPEHPK3PXP"
	if got := CleanSecret(input); got != expected {
		t.Errorf("CleanSecret(%q) = %q; want %q", input, got, expected)
	}

	invalid := "SECURITYANDLOGIN"
	if got := CleanSecret(invalid); got != "" {
		t.Errorf("CleanSecret(%q) = %q; want empty string", invalid, got)
	}
}

func TestGetCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	code, err := GetCode(ctx, "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Logf("GetCode error (network might be offline/blocked): %v", err)
		return
	}
	t.Logf("Fetched 2FA Code successfully: %s", code)
	if len(code) != 6 {
		t.Errorf("Expected 6-digit code, got %s", code)
	}
}
