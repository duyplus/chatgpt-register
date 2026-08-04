package handlers

import (
	"testing"
)

func TestNormalizeProxy(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3.4:8080", "http://1.2.3.4:8080"},
		{"  192.168.1.1:1080  ", "http://192.168.1.1:1080"},
		{"http://1.2.3.4:8080", "http://1.2.3.4:8080"},
		{"socks5://1.2.3.4:1080", "socks5://1.2.3.4:1080"},
		{"1.2.3.4:8080:usr:pwd", "http://usr:pwd@1.2.3.4:8080"},
		{"usr:pwd:1.2.3.4:8080", "http://usr:pwd@1.2.3.4:8080"},
		{"http://usr:pwd@1.2.3.4:8080", "http://usr:pwd@1.2.3.4:8080"},
	}

	for _, tt := range tests {
		got := normalizeProxy(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeProxy(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
