package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"chatgpt-register/internal/models"

	"github.com/gin-gonic/gin"
)

// accessToken 从库里存的 auth.json 提取 access_token。
func accessToken(authData string) string {
	var parsed map[string]any
	_ = json.Unmarshal([]byte(authData), &parsed)
	s, _ := parsed["access_token"].(string)
	return s
}

// Produce 启动一次生产：{ "count": N }。
func (h *Handler) Produce(c *gin.Context) {
	var in struct {
		Count int `json:"count"`
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
	if err := h.Producer.Start(in.Count); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ProduceStatus 返回生产进度（待生产/在跑/已注册/失败/日志）。
func (h *Handler) ProduceStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.Producer.Snapshot())
}

// BrowserStatus 返回 rod 浏览器的下载/就绪状态，供仪表盘展示进度。
func (h *Handler) BrowserStatus(c *gin.Context) {
	if h.Browser == nil {
		c.JSON(http.StatusOK, gin.H{"ready": true, "phase": "ready"})
		return
	}
	c.JSON(http.StatusOK, h.Browser.Snapshot())
}

// ProduceStop 停止生产。
func (h *Handler) ProduceStop(c *gin.Context) {
	h.Producer.Stop()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegistrationLog 返回单个账号的执行日志。
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

// RegistrationShot 返回单个账号注册失败时保存的页面截图(PNG)。
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

// SetShipped 禁止手动切换出库状态。
// 出库状态只能由下载接口自动标记，避免库存状态被人工改乱。
func (h *Handler) SetShipped(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "Shipped status is locked; it can only be updated automatically by downloading"})
}

// Download 导出选中账号的 access_token：纯文本，一行一个；下载即标记出库。
// 请求体：{ "ids": [1,2,3] }。
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
	if err := h.DB.Where("id IN ? AND status = ? AND auth_data <> ''", in.IDs, "registered").
		Find(&regs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(regs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected accounts have no downloadable registered data"})
		return
	}

	tokens := make([]string, 0, len(regs))
	ids := make([]uint, 0, len(regs))
	for _, r := range regs {
		if tok := accessToken(r.AuthData); tok != "" {
			tokens = append(tokens, tok)
		}
		ids = append(ids, r.ID)
	}
	if len(tokens) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected accounts lack access_token"})
		return
	}

	// 下载即出库
	h.DB.Model(&models.Registration{}).Where("id IN ?", ids).Update("shipped", true)

	c.Header("Content-Disposition", "attachment; filename=access_tokens.txt")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strings.Join(tokens, "\n")+"\n"))
}
