package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"chatgpt-register/internal/auth"
	"chatgpt-register/internal/browserboot"
	"chatgpt-register/internal/mailfetch"
	"chatgpt-register/internal/models"
	"chatgpt-register/internal/producer"
	"chatgpt-register/internal/twofactor"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	DB       *gorm.DB
	Mail     *mailfetch.Client
	Auth     *auth.Service
	Producer *producer.Producer
	Browser  *browserboot.Manager
}

func New(db *gorm.DB, authSvc *auth.Service, browser *browserboot.Manager) *Handler {
	mail := mailfetch.New()
	return &Handler{DB: db, Mail: mail, Auth: authSvc, Producer: producer.New(db, mail), Browser: browser}
}

type registrationInput struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

func validStatus(s string) bool {
	return s == "" || s == "pending" || s == "registering" || s == "logging_in" ||
		s == "registered" || s == "register_failed" || s == "already_registered"
}

func (h *Handler) List(c *gin.Context) {
	var regs []models.Registration
	q := h.DB.Model(&models.Registration{})

	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if kw := c.Query("q"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("email LIKE ? OR note LIKE ?", like, like)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := q.Order("mailbox_id asc, id asc").Offset((page - 1) * size).Limit(size).Find(&regs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// List does not return auth_data / log (contains private keys, large size), returned on-demand in download/log endpoints
	for i := range regs {
		regs[i].AuthData = ""
		regs[i].Log = ""
	}
	c.JSON(http.StatusOK, gin.H{"data": regs, "total": total, "page": page, "size": size})
}

func (h *Handler) Get(c *gin.Context) {
	var reg models.Registration
	if err := h.DB.First(&reg, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	// Do not return private keys/full auth in general endpoint, returned on-demand in download endpoint
	reg.AuthData = ""
	c.JSON(http.StatusOK, reg)
}

func (h *Handler) Create(c *gin.Context) {
	var in registrationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validStatus(in.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	reg := models.Registration{
		Email:    in.Email,
		Password: in.Password,
		Status:   in.Status,
		Note:     in.Note,
	}
	if reg.Status == "" {
		reg.Status = "pending"
	}
	if err := h.DB.Create(&reg).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, reg)
}

func (h *Handler) Update(c *gin.Context) {
	var reg models.Registration
	if err := h.DB.First(&reg, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var in registrationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validStatus(in.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	reg.Email = in.Email
	reg.Password = in.Password
	if in.Status != "" {
		reg.Status = in.Status
	}
	reg.Note = in.Note
	if err := h.DB.Save(&reg).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reg)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.DB.Delete(&models.Registration{}, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) Stats(c *gin.Context) {
	reg := func(where ...any) int64 {
		var n int64
		q := h.DB.Model(&models.Registration{})
		if len(where) > 0 {
			q = q.Where(where[0], where[1:]...)
		}
		q.Count(&n)
		return n
	}

	total := reg()
	pending := reg("status = ?", "pending")
	registering := reg("status = ?", "registering")
	registered := reg("status = ?", "registered")
	registerFailed := reg("status = ?", "register_failed")
	shipped := reg("shipped = ?", true)

	var mailboxes, mailboxVerified int64
	h.DB.Model(&models.Mailbox{}).Count(&mailboxes)
	h.DB.Model(&models.Mailbox{}).Where("status = ?", "verified").Count(&mailboxVerified)

	// Registered but unshipped stock
	unshipped := registered - shipped
	if unshipped < 0 {
		unshipped = 0
	}

	// Plan type distribution
	type kv struct {
		PlanType string
		N        int64
	}
	var plans []kv
	h.DB.Model(&models.Registration{}).
		Select("plan_type, count(*) as n").
		Where("status = ? AND plan_type <> ''", "registered").
		Group("plan_type").Scan(&plans)
	planBreak := make(map[string]int64, len(plans))
	for _, p := range plans {
		planBreak[p.PlanType] = p.N
	}

	// Past 7 days registered yield trend
	type day struct {
		D string
		N int64
	}
	var days []day
	h.DB.Model(&models.Registration{}).
		Select("strftime('%Y-%m-%d', created_at) as d, count(*) as n").
		Where("status = ?", "registered").
		Group("d").Order("d desc").Limit(7).Scan(&days)
	trend := make([]gin.H, 0, len(days))
	for i := len(days) - 1; i >= 0; i-- {
		trend = append(trend, gin.H{"date": days[i].D, "count": days[i].N})
	}

	prog := h.Producer.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"total": total, "pending": pending, "registering": registering,
		"registered": registered, "register_failed": registerFailed,
		"shipped": shipped, "unshipped": unshipped,
		"mailboxes": mailboxes, "mailbox_verified": mailboxVerified,
		"plans": planBreak, "trend": trend,
		"running": prog.Running, "produce_target": prog.Target,
		"produce_pending": prog.Pending, "produce_running": prog.RunningNum,
		"produce_registered": prog.Registered, "produce_failed": prog.Failed,
		"produce_message": prog.Message,
	})
}

// RegistrationTwoFactorCode Fetches 2FA code for a registration account using 2fa.live
func (h *Handler) RegistrationTwoFactorCode(c *gin.Context) {
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
