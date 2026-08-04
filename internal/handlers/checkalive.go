package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chatgpt-register/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const usageLimitsURL = "https://chatgpt.com/backend-api/pageConfigs/usage_limits"

// checkAliveUA User-Agent for check alive requests to avoid Cloudflare blocking empty UA.
const checkAliveUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// chatgptAccountID Extracts chatgpt_account_id from access_token (JWT) for chatgpt-account-id header.
// Performs base64url payload decoding only, signature validation is skipped.
func chatgptAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Auth struct {
			AccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.Auth.AccountID
}

// probeAlive Requests usage_limits using access_token to determine account liveness.
// Returns: alive=200 alive; transportErr=network/timeout transport error (detection error, status unchanged);
// other non-200 considered invalid (disabled). code is HTTP status code, msg is readable reason.
func probeAlive(token, accountID string) (alive bool, transportErr bool, code int, msg string) {
	req, err := http.NewRequest(http.MethodGet, usageLimitsURL, nil)
	if err != nil {
		return false, true, 0, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", checkAliveUA)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://chatgpt.com/")
	if accountID != "" {
		req.Header.Set("chatgpt-account-id", accountID)
	}

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return false, true, 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode == http.StatusOK {
		return true, false, resp.StatusCode, "alive"
	}
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	return false, false, resp.StatusCode, snippet
}

// CheckAlive Checks liveness for selected accounts: requests usage_limits with access_token.
func (h *Handler) CheckAlive(c *gin.Context) {
	var ids []uint
	if idParam := c.Param("id"); idParam != "" {
		n, err := strconv.ParseUint(idParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		ids = []uint{uint(n)}
	} else {
		var in struct {
			IDs []uint `json:"ids"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ids = in.IDs
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No accounts selected"})
		return
	}

	var regs []models.Registration
	if err := h.DB.Where("id IN ?", ids).Find(&regs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var aliveN, deadN, errN int
	results := make([]gin.H, 0, len(regs))
	for i := range regs {
		reg := &regs[i]
		tok := accessToken(reg.AuthData)
		if tok == "" {
			errN++
			results = append(results, gin.H{"id": reg.ID, "email": reg.Email, "result": "error", "message": "Missing access_token"})
			continue
		}

		ok, transportErr, statusCode, msg := probeAlive(tok, chatgptAccountID(tok))
		switch {
		case ok:
			aliveN++
			// If previously disabled and alive now, restore to registered
			if reg.Status != "registered" {
				h.appendLog(reg, fmt.Sprintf("✔ Alive check passed (HTTP %d), restored status to registered", statusCode))
				h.DB.Model(reg).Update("status", "registered")
			}
			results = append(results, gin.H{"id": reg.ID, "email": reg.Email, "result": "alive", "code": statusCode})
		case transportErr:
			errN++
			results = append(results, gin.H{"id": reg.ID, "email": reg.Email, "result": "error", "code": statusCode, "message": msg})
		default:
			deadN++
			h.appendLog(reg, fmt.Sprintf("✗ Alive check failed (HTTP %d), disabled: %s", statusCode, msg))
			h.DB.Model(reg).Update("status", "already_registered")
			results = append(results, gin.H{"id": reg.ID, "email": reg.Email, "result": "dead", "code": statusCode, "message": msg})
		}
	}

	c.JSON(http.StatusOK, gin.H{"alive": aliveN, "dead": deadN, "error": errN, "results": results})
}

// appendLog Appends a timestamped log entry to the account.
func (h *Handler) appendLog(reg *models.Registration, line string) {
	entry := "\n[" + time.Now().Format("02/01/2006 15:04:05") + "] " + line
	h.DB.Model(reg).Update("log", gorm.Expr("log || ?", entry))
}
