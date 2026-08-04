package handlers

import (
	"net/http"
	"strings"

	"chatgpt-register/internal/auth"

	"github.com/gin-gonic/gin"
)

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/login → {token, username}
func (h *Handler) Login(c *gin.Context) {
	var in loginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parameter"})
		return
	}
	if in.Username == "" {
		in.Username = auth.DefaultUser
	}
	tok, a, err := h.Auth.Login(in.Username, in.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok, "username": a.Username})
}

type changePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword POST /api/change-password → {token}
func (h *Handler) ChangePassword(c *gin.Context) {
	var in changePasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parameter"})
		return
	}
	adminID := c.GetUint("admin_id")
	tok, err := h.Auth.ChangePassword(adminID, in.OldPassword, in.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": tok})
}

// AuthRequired middleware: validates Bearer token; if renewed, issues new token via X-New-Token response header.
func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		tok := strings.TrimPrefix(raw, "Bearer ")
		a, newTok, err := h.Auth.Validate(tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if newTok != "" {
			c.Header("X-New-Token", newTok)
		}
		c.Set("admin_id", a.ID)
		c.Next()
	}
}
