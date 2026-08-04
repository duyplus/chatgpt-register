package handlers

import (
	"net/http"
	"strings"

	"chatgpt-register/internal/varymail"

	"github.com/gin-gonic/gin"
)


// VarymailServices Queries chatgpt service stock for settings page key verification / stock checking.
// Supports overriding with key in query string temporarily (testing before saving).
func (h *Handler) VarymailServices(c *gin.Context) {
	key := strings.TrimSpace(c.Query("key"))
	if key == "" {
		key = h.setting("varymail_api_key")
	}
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in the varymail API Key first"})
		return
	}

	cli := varymail.New("", key)
	svc, price, err := cli.ServiceByName(c.Request.Context(), varymail.DefaultServiceName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"service": svc, "price": price})
}
