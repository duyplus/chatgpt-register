package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"chatgpt-register/internal/auth"
	"chatgpt-register/internal/browserboot"
	"chatgpt-register/internal/db"
	"chatgpt-register/internal/handlers"

	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFS embed.FS

func main() {
	database, err := db.Init("adskull.db")
	if err != nil {
		log.Fatalf("init db: %v", err)
	}

	// Records left in "registering" status after restart have no surviving task, marked as failed.
	if err := database.Exec(
		"UPDATE registrations SET status = 'register_failed', log = log || ? WHERE status = 'registering'",
		"\n["+time.Now().Format("02/01/2006 15:04:05")+"] ✗ Program restarted, task interrupted, marked as failed",
	).Error; err != nil {
		log.Printf("reset registering on boot: %v", err)
	}

	authSvc, err := auth.New(database)
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}

	// Ensure the browser required by rod is ready asynchronously on startup, auto download if not ready.
	browser := browserboot.New()
	browser.EnsureAsync()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	h := handlers.New(database, authSvc, browser)

	r.POST("/api/login", h.Login)

	api := r.Group("/api", h.AuthRequired())
	{
		api.POST("/change-password", h.ChangePassword)

		api.GET("/stats", h.Stats)
		api.GET("/registrations", h.List)
		api.GET("/registrations/:id", h.Get)
		api.POST("/registrations", h.Create)
		api.PUT("/registrations/:id", h.Update)
		api.DELETE("/registrations/:id", h.Delete)
		api.GET("/registrations/:id/logs", h.RegistrationLog)
		api.GET("/registrations/:id/shot", h.RegistrationShot)
		api.GET("/registrations/:id/2fa", h.RegistrationTwoFactorCode)
		api.PUT("/registrations/:id/shipped", h.SetShipped)
		api.POST("/download", h.Download)

		api.POST("/registrations/:id/check-alive", h.CheckAlive)
		api.POST("/check-alive", h.CheckAlive)

		api.POST("/produce", h.Produce)
		api.GET("/produce/status", h.ProduceStatus)
		api.POST("/produce/stop", h.ProduceStop)
		api.GET("/browser/status", h.BrowserStatus)

		api.GET("/mailboxes", h.MailboxList)
		api.POST("/mailboxes", h.MailboxCreate)
		api.POST("/mailboxes/import", h.MailboxImport)
		api.POST("/mailboxes/:id/verify", h.MailboxVerify)
		api.PUT("/mailboxes/:id", h.MailboxUpdate)
		api.DELETE("/mailboxes/:id", h.MailboxDelete)
		api.GET("/mailboxes/:id/messages", h.MailboxMessages)
		api.GET("/mailboxes/:id/message", h.MailboxMessage)
		api.GET("/mailboxes/:id/2fa", h.MailboxTwoFactorCode)
		api.GET("/2fa/code", h.MailboxTwoFactorCode)

		api.GET("/settings", h.SettingsGet)
		api.PUT("/settings", h.SettingsSave)

		api.GET("/varymail/services", h.VarymailServices)

		api.POST("/proxy/test", h.ProxyTest)

		// DB Manager API routes
		api.GET("/db/tables", h.GetDBTables)
		api.GET("/db/rows", h.GetTableRows)
		api.POST("/db/rows", h.CreateDBRecord)
		api.PUT("/db/rows", h.UpdateDBRecord)
		api.DELETE("/db/rows", h.DeleteDBRecord)
		api.POST("/db/truncate", h.TruncateDBTable)
		api.GET("/db/backup", h.BackupDB)
		api.POST("/db/restore", h.RestoreDB)
	}

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	httpFS := http.FS(sub)
	r.StaticFS("/static", httpFS)
	for _, p := range []string{"login", "dashboard", "mailboxes", "accounts", "settings", "db"} {
		p := p
		r.GET("/"+p, func(c *gin.Context) { c.FileFromFS(p+".html", httpFS) })
	}
	r.GET("/", func(c *gin.Context) { c.FileFromFS("dashboard.html", httpFS) })

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":9000"
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	log.Printf("chatgpt-register listening on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
