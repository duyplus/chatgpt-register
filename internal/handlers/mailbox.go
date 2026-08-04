package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"chatgpt-register/internal/emailalias"
	"chatgpt-register/internal/mailfetch"
	"chatgpt-register/internal/models"
	"chatgpt-register/internal/twofactor"
	"chatgpt-register/internal/varymail"

	"github.com/gin-gonic/gin"
)

type mailboxInput struct {
	Email        string `json:"email" binding:"required"`
	Password     string `json:"password"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
	Provider     string `json:"provider"`
	Status       string `json:"status"`
	Note         string `json:"note"`
}

var mailboxStatuses = map[string]bool{
	"unverified":    true,
	"verifying":     true,
	"verify_failed": true,
	"verified":      true,
}

func validMailboxStatus(s string) bool {
	return s == "" || mailboxStatuses[s]
}

func (h *Handler) MailboxList(c *gin.Context) {
	var items []models.Mailbox
	q := h.DB.Order("id asc")
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if kw := c.Query("q"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("email LIKE ? OR provider LIKE ? OR note LIKE ?", like, like, like)
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	} else if size > 10000 {
		size = 10000
	}
	var total int64
	q.Model(&models.Mailbox{}).Count(&total)
	if err := q.Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	registerLimit := 1
	for i := range items {
		items[i].RegisterCount = h.mailboxRegisterCount(items[i])
		items[i].RegisterLimit = registerLimit
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "size": size})
}

func (h *Handler) mailboxRegisterCount(m models.Mailbox) int {
	var n int64
	// Records belonging to this mailbox: direct address or mailbox_id (including legacy alias entries)
	match := h.DB.Where("mailbox_id = ? OR email = ?", m.ID, m.Email)
	if pattern := emailalias.LikePattern(m.Email); pattern != "" {
		match = match.Or("email LIKE ? ESCAPE '\\'", pattern)
	}
	// Only count records that occupy quota (registered / already_registered), pending / failed are excluded
	h.DB.Model(&models.Registration{}).
		Where(match).
		Where("status IN ?", []string{"registered", "already_registered"}).
		Count(&n)
	return int(n)
}

func (h *Handler) MailboxCreate(c *gin.Context) {
	var in mailboxInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validMailboxStatus(in.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	m := models.Mailbox{
		Email:        in.Email,
		Password:     in.Password,
		Provider:     in.Provider,
		ClientID:     in.ClientID,
		RefreshToken: in.RefreshToken,
		Status:       in.Status,
		Note:         in.Note,
	}
	if m.Status == "" {
		m.Status = "unverified"
	}
	if err := h.DB.Create(&m).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

type mailboxImportItem struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

// MailboxImport Batch imports mailboxes, duplicate emails are skipped.
func (h *Handler) MailboxImport(c *gin.Context) {
	var in struct {
		Items []mailboxImportItem `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	added, skipped := 0, 0
	seen := map[string]bool{}
	for _, it := range in.Items {
		email := strings.TrimSpace(it.Email)
		if email == "" || !strings.Contains(email, "@") || seen[email] {
			skipped++
			continue
		}
		seen[email] = true
		var count int64
		h.DB.Model(&models.Mailbox{}).Where("email = ?", email).Count(&count)
		if count > 0 {
			skipped++
			continue
		}
		m := models.Mailbox{
			Email:        email,
			Password:     strings.TrimSpace(it.Password),
			ClientID:     strings.TrimSpace(it.ClientID),
			RefreshToken: strings.TrimSpace(it.RefreshToken),
			Status:       "verifying",
		}
		if err := h.DB.Create(&m).Error; err != nil {
			skipped++
			continue
		}
		added++
	}
	c.JSON(http.StatusOK, gin.H{"added": added, "skipped": skipped})
}

// MailboxVerify Validates single mailbox credential, updating status to verified / verify_failed.
func (h *Handler) MailboxVerify(c *gin.Context) {
	var m models.Mailbox
	if err := h.DB.First(&m, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mailbox not found"})
		return
	}
	if m.Provider == "varymail" {
		var err error
		if key := h.setting("varymail_api_key"); key == "" || m.PurchaseID <= 0 {
			err = varymail.ErrUnauthorized
		} else {
			_, _, err = varymail.New("", key).Code(c.Request.Context(), m.PurchaseID)
		}
		if err != nil {
			m.Status = "verify_failed"
		} else {
			m.Status = "verified"
		}
		h.DB.Model(&m).Update("status", m.Status)
		c.JSON(http.StatusOK, gin.H{"id": m.ID, "status": m.Status})
		return
	}
	err := h.Mail.Verify(c.Request.Context(), mailfetch.Account{
		Email:        m.Email,
		ClientID:     m.ClientID,
		RefreshToken: m.RefreshToken,
	})
	if err != nil {
		m.Status = "verify_failed"
		m.Note = err.Error()
		h.DB.Model(&m).Updates(map[string]any{"status": m.Status, "note": m.Note})
		c.JSON(http.StatusOK, gin.H{"id": m.ID, "status": m.Status, "error": err.Error()})
		return
	}
	m.Status = "verified"
	h.DB.Model(&m).Updates(map[string]any{"status": m.Status, "note": ""})
	c.JSON(http.StatusOK, gin.H{"id": m.ID, "status": m.Status})
}

// MailboxTwoFactorCode Fetches 2FA code for a registration account or custom secret using https://2fa.live/
func (h *Handler) MailboxTwoFactorCode(c *gin.Context) {
	secret := strings.TrimSpace(c.Query("secret"))
	if secret == "" {
		id := c.Param("id")
		if id != "" {
			var r models.Registration
			if err := h.DB.First(&r, id).Error; err == nil {
				secret = r.TwoFactorSecret
			}
		}
	}
	if secret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No 2FA secret provided"})
		return
	}
	code, err := twofactor.GetCode(c.Request.Context(), secret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code, "secret": twofactor.CleanSecret(secret)})
}

func (h *Handler) MailboxUpdate(c *gin.Context) {
	var m models.Mailbox
	if err := h.DB.First(&m, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mailbox not found"})
		return
	}
	var in mailboxInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validMailboxStatus(in.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	m.Email = in.Email
	m.Password = in.Password
	m.Provider = in.Provider
	m.ClientID = in.ClientID
	m.RefreshToken = in.RefreshToken
	if in.Status != "" {
		m.Status = in.Status
	}
	m.Note = in.Note
	if err := h.DB.Save(&m).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) MailboxDelete(c *gin.Context) {
	if err := h.DB.Delete(&models.Mailbox{}, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MailboxMessages Fetch latest inbox messages for mailbox.
func (h *Handler) MailboxMessages(c *gin.Context) {
	var m models.Mailbox
	if err := h.DB.First(&m, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mailbox not found"})
		return
	}
	if m.Provider == "varymail" {
		h.varymailMessages(c, m)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	msgs, err := h.Mail.ListMessages(c.Request.Context(), mailfetch.Account{
		Email:        m.Email,
		ClientID:     m.ClientID,
		RefreshToken: m.RefreshToken,
	}, limit)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, mailfetch.ErrMissingCreds) || errors.Is(err, mailfetch.ErrAuthFailed) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error(), "email": m.Email})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": m.Email, "items": msgs})
}

// MailboxMessage Fetch full content of a single email message by ID.
func (h *Handler) MailboxMessage(c *gin.Context) {
	var m models.Mailbox
	if err := h.DB.First(&m, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mailbox not found"})
		return
	}
	if m.Provider == "varymail" {
		h.varymailMessage(c, m)
		return
	}
	msg, err := h.Mail.GetMessage(c.Request.Context(), mailfetch.Account{
		Email:        m.Email,
		ClientID:     m.ClientID,
		RefreshToken: m.RefreshToken,
	}, c.Query("mid"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, mailfetch.ErrMissingCreds) || errors.Is(err, mailfetch.ErrAuthFailed) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error(), "email": m.Email})
		return
	}
	c.JSON(http.StatusOK, msg)
}

// varymailMessages Fetch latest email via varymail purchase ID.
func (h *Handler) varymailMessages(c *gin.Context, m models.Mailbox) {
	key := h.setting("varymail_api_key")
	if key == "" || m.PurchaseID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing varymail API Key or purchase ID", "email": m.Email})
		return
	}
	msg, hasMail, err := varymail.New("", key).Code(c.Request.Context(), m.PurchaseID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "email": m.Email})
		return
	}
	items := []gin.H{}
	if hasMail {
		items = append(items, gin.H{
			"id": msg.ID, "from": msg.From, "from_name": "",
			"subject": "Code: " + msg.Code, "received_at": msg.ReceivedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"email": m.Email, "items": items})
}

// varymailMessage varymail only has verification codes, returns code text as body.
func (h *Handler) varymailMessage(c *gin.Context, m models.Mailbox) {
	key := h.setting("varymail_api_key")
	if key == "" || m.PurchaseID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing varymail API Key or purchase ID", "email": m.Email})
		return
	}
	msg, hasMail, err := varymail.New("", key).Code(c.Request.Context(), m.PurchaseID)
	if err != nil || !hasMail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No mail available", "email": m.Email})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": msg.ID, "from": msg.From, "subject": "Code: " + msg.Code, "text": "Code: " + msg.Code, "received_at": msg.ReceivedAt})
}
