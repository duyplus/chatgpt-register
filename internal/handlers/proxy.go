package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/proxy"
)

type proxyTestInput struct {
	Proxy string `json:"proxy"`
}

func isPortStr(s string) bool {
	p, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && p > 0 && p <= 65535
}

// normalizeProxy converts formats like ip:port, host:port:user:pass, user:pass:host:port to standard URL; returns as-is if scheme exists.
func normalizeProxy(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		return raw
	}
	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 2: // ip:port or host:port
		return "http://" + parts[0] + ":" + parts[1]
	case 4: // host:port:user:pass or user:pass:host:port
		if isPortStr(parts[1]) {
			return "http://" + url.QueryEscape(parts[2]) + ":" + url.QueryEscape(parts[3]) + "@" + parts[0] + ":" + parts[1]
		} else if isPortStr(parts[3]) {
			return "http://" + url.QueryEscape(parts[0]) + ":" + url.QueryEscape(parts[1]) + "@" + parts[2] + ":" + parts[3]
		}
		return "http://" + url.QueryEscape(parts[2]) + ":" + url.QueryEscape(parts[3]) + "@" + parts[0] + ":" + parts[1]
	default:
		return "http://" + raw
	}
}

// ProxyTest Requests IP detection service via given proxy to return exit IP, validating proxy usability.
func (h *Handler) ProxyTest(c *gin.Context) {
	var in proxyTestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pu := normalizeProxy(in.Proxy)
	if pu == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Proxy is empty"})
		return
	}
	u, err := url.Parse(pu)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "Invalid proxy format"})
		return
	}

	transport := &http.Transport{}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h", "socks4", "socks4a", "socks":
		dialer, derr := proxy.FromURL(u, proxy.Direct)
		if derr != nil {
			var auth *proxy.Auth
			if u.User != nil {
				pw, _ := u.User.Password()
				auth = &proxy.Auth{User: u.User.Username(), Password: pw}
			}
			dialer, derr = proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
		}
		if derr != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": derr.Error()})
			return
		}
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = cd.DialContext
		} else {
			transport.Dial = dialer.Dial //nolint:staticcheck
		}
	default:
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "Unsupported proxy type: " + u.Scheme})
		return
	}

	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	start := time.Now()

	// Try HTTPS first with browser User-Agent, then fallback to HTTP IP services if proxy lacks SSL CONNECT support
	ip, err := testProxyRequest(client, "https://api.ipify.org?format=text")
	if err != nil {
		ip, err = testProxyRequest(client, "http://api.ipify.org?format=text")
	}
	if err != nil {
		ip, err = testProxyRequest(client, "http://ip-api.com/line/?fields=query")
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "ip": ip, "ms": time.Since(start).Milliseconds()})
}

func testProxyRequest(client *http.Client, targetURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty IP response")
	}
	return ip, nil
}
