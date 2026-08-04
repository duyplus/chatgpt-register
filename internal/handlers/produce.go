package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"chatgpt-register/internal/models"

	"github.com/gin-gonic/gin"
)

// accessToken extracts access_token from stored auth.json in DB.
func accessToken(authData string) string {
	var parsed map[string]any
	_ = json.Unmarshal([]byte(authData), &parsed)
	s, _ := parsed["access_token"].(string)
	return s
}

// Produce Starts a production task: { "count": N, "login_only": bool, "mailbox_ids": []uint }.
func (h *Handler) Produce(c *gin.Context) {
	var in struct {
		Count      int    `json:"count"`
		LoginOnly  bool   `json:"login_only"`
		MailboxIDs []uint `json:"mailbox_ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.Browser == nil || !h.Browser.Ready() {
		c.JSON(http.StatusConflict, gin.H{"error": "Browser missing, cannot produce: browser is downloading or download failed"})
		return
	}
	if h.setting("email_source") == "varymail" {
		if h.setting("varymail_api_key") == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "varymail selected, but API Key is missing. Please configure it in settings first."})
			return
		}
	}
	if err := h.Producer.Start(in.Count, in.LoginOnly, in.MailboxIDs); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ProduceStatus Returns production progress snapshot.
func (h *Handler) ProduceStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.Producer.Snapshot())
}

// BrowserStatus Returns rod browser download/ready status for dashboard progress.
func (h *Handler) BrowserStatus(c *gin.Context) {
	if h.Browser == nil {
		c.JSON(http.StatusOK, gin.H{"ready": true, "phase": "ready"})
		return
	}
	c.JSON(http.StatusOK, h.Browser.Snapshot())
}

// ProduceStop Stops production.
func (h *Handler) ProduceStop(c *gin.Context) {
	h.Producer.Stop()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegistrationLog Returns execution log for a single account.
func (h *Handler) RegistrationLog(c *gin.Context) {
	var reg models.Registration
	if err := h.DB.First(&reg, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"email": reg.Email, "status": reg.Status,
		"note": reg.Note, "log": reg.Log,
		"has_shot": len(reg.Shot) > 0,
	})
}

// RegistrationShot Returns page screenshot (PNG) saved on registration failure for a single account.
func (h *Handler) RegistrationShot(c *gin.Context) {
	var reg models.Registration
	if err := h.DB.First(&reg, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if len(reg.Shot) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No screenshot available"})
		return
	}
	c.Data(http.StatusOK, "image/png", reg.Shot)
}

// SetShipped Disallows manually switching shipped status.
// Shipped status can only be updated automatically by downloading.
func (h *Handler) SetShipped(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "Shipped status is locked; it can only be updated automatically by downloading"})
}

// Download Exports selected accounts in mail|password|2fa format: plain text, one per line; downloading marks as shipped.
// Request body: { "ids": [1,2,3] }.
func (h *Handler) Download(c *gin.Context) {
	var in struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(in.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No accounts selected"})
		return
	}

	var regs []models.Registration
	if err := h.DB.Where("id IN ? AND status = ?", in.IDs, "registered").
		Find(&regs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(regs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected accounts have no downloadable registered data"})
		return
	}

	lines := make([]string, 0, len(regs))
	ids := make([]uint, 0, len(regs))
	for _, r := range regs {
		secret := strings.TrimSpace(r.TwoFactorSecret)
		lines = append(lines, strings.TrimSpace(r.Email)+"|"+strings.TrimSpace(r.Password)+"|"+secret)
		ids = append(ids, r.ID)
	}

	// Downloading marks as shipped
	h.DB.Model(&models.Registration{}).Where("id IN ?", ids).Update("shipped", true)

	filename := fmt.Sprintf("account_%d.txt", len(regs))
	if len(regs) == 1 {
		prefix := strings.Split(regs[0].Email, "@")[0]
		if prefix != "" {
			filename = fmt.Sprintf("account_%s.txt", prefix)
		}
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strings.Join(lines, "\n")+"\n"))
}
