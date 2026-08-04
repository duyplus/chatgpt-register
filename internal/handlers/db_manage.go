package handlers

import (
	"chatgpt-register/internal/models"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var allowedTables = map[string]bool{
	"registrations": true,
	"mailboxes":     true,
	"settings":      true,
	"admins":         true,
}

var tablePrimaryKeys = map[string]string{
	"registrations": "id",
	"mailboxes":     "id",
	"settings":      "key",
	"admins":        "id",
}

var tableSchemas = map[string][]gin.H{
	"registrations": {
		{"name": "id", "type": "number", "readonly": true, "primaryKey": true},
		{"name": "mailbox_id", "type": "number", "readonly": false},
		{"name": "email", "type": "string", "readonly": false},
		{"name": "password", "type": "string", "readonly": false},
		{"name": "two_factor_secret", "type": "string", "readonly": false},
		{"name": "status", "type": "string", "readonly": false},
		{"name": "shipped", "type": "boolean", "readonly": false},
		{"name": "note", "type": "string", "readonly": false},
		{"name": "created_at", "type": "datetime", "readonly": true},
		{"name": "updated_at", "type": "datetime", "readonly": true},
	},
	"mailboxes": {
		{"name": "id", "type": "number", "readonly": true, "primaryKey": true},
		{"name": "email", "type": "string", "readonly": false},
		{"name": "password", "type": "string", "readonly": false},
		{"name": "client_id", "type": "string", "readonly": false},
		{"name": "refresh_token", "type": "string", "readonly": false},
		{"name": "provider", "type": "string", "readonly": false},
		{"name": "status", "type": "string", "readonly": false},
		{"name": "note", "type": "string", "readonly": false},
		{"name": "created_at", "type": "datetime", "readonly": true},
		{"name": "updated_at", "type": "datetime", "readonly": true},
	},
	"settings": {
		{"name": "key", "type": "string", "readonly": false, "primaryKey": true},
		{"name": "value", "type": "string", "readonly": false},
		{"name": "updated_at", "type": "datetime", "readonly": true},
	},
	"admins": {
		{"name": "id", "type": "number", "readonly": true, "primaryKey": true},
		{"name": "username", "type": "string", "readonly": false},
		{"name": "password_hash", "type": "string", "readonly": false},
		{"name": "created_at", "type": "datetime", "readonly": true},
		{"name": "updated_at", "type": "datetime", "readonly": true},
	},
}

// GetDBTables Returns available database tables and schemas
func (h *Handler) GetDBTables(c *gin.Context) {
	tables := []gin.H{
		{"name": "registrations", "label": "Registrations", "primaryKey": "id", "schema": tableSchemas["registrations"]},
		{"name": "mailboxes", "label": "Mailboxes", "primaryKey": "id", "schema": tableSchemas["mailboxes"]},
		{"name": "settings", "label": "Settings", "primaryKey": "key", "schema": tableSchemas["settings"]},
		{"name": "admins", "label": "Admins", "primaryKey": "id", "schema": tableSchemas["admins"]},
	}
	c.JSON(http.StatusOK, gin.H{"tables": tables})
}

// GetTableRows Returns paginated records for a selected table
func (h *Handler) GetTableRows(c *gin.Context) {
	table := strings.TrimSpace(c.Query("table"))
	if !allowedTables[table] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
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
	q := h.DB.Table(table)

	if kw := strings.TrimSpace(c.Query("q")); kw != "" {
		like := "%" + kw + "%"
		switch table {
		case "registrations":
			q = q.Where("email LIKE ? OR username LIKE ? OR note LIKE ? OR status LIKE ?", like, like, like, like)
		case "mailboxes":
			q = q.Where("email LIKE ? OR type LIKE ? OR status LIKE ?", like, like, like)
		case "settings":
			q = q.Where("key LIKE ? OR value LIKE ?", like, like)
		case "admins":
			q = q.Where("username LIKE ?", like)
		}
	}

	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	orderClause := "id asc"
	if table == "settings" {
		orderClause = "key asc"
	} else if table == "registrations" {
		orderClause = "mailbox_id asc, id asc"
	}

	var rows []map[string]any
	if err := q.Order(orderClause).Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, row := range rows {
		for k, v := range row {
			if str, ok := v.(string); ok && (strings.HasSuffix(k, "_at") || k == "created_at" || k == "updated_at") {
				row[k] = models.FormatShortTime(str)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"table":      table,
		"primaryKey": tablePrimaryKeys[table],
		"total":      total,
		"page":       page,
		"size":       size,
		"rows":       rows,
		"schema":     tableSchemas[table],
	})
}

// CreateDBRecord Inserts a new record into a table
func (h *Handler) CreateDBRecord(c *gin.Context) {
	var input struct {
		Table string         `json:"table" binding:"required"`
		Data  map[string]any `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	table := strings.TrimSpace(input.Table)
	if !allowedTables[table] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	delete(input.Data, "id")
	delete(input.Data, "created_at")
	delete(input.Data, "updated_at")

	if err := h.DB.Table(table).Create(&input.Data).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record created successfully", "record": input.Data})
}

// UpdateDBRecord Updates an existing record in a table by ID/Key
func (h *Handler) UpdateDBRecord(c *gin.Context) {
	var input struct {
		Table string         `json:"table" binding:"required"`
		ID    any            `json:"id" binding:"required"`
		Data  map[string]any `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	table := strings.TrimSpace(input.Table)
	if !allowedTables[table] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	pkName := tablePrimaryKeys[table]
	if pkName == "" {
		pkName = "id"
	}

	delete(input.Data, "id")
	delete(input.Data, "created_at")
	delete(input.Data, "updated_at")

	if err := h.DB.Table(table).Where(pkName+" = ?", input.ID).Updates(input.Data).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record updated successfully"})
}

// DeleteDBRecord Deletes a record from a table by ID/Key
func (h *Handler) DeleteDBRecord(c *gin.Context) {
	var input struct {
		Table string `json:"table" binding:"required"`
		ID    any    `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	table := strings.TrimSpace(input.Table)
	if !allowedTables[table] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	pkName := tablePrimaryKeys[table]
	if pkName == "" {
		pkName = "id"
	}

	if err := h.DB.Table(table).Where(pkName+" = ?", input.ID).Delete(nil).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Record deleted successfully"})
}

// TruncateDBTable Deletes all records from a table and resets autoincrement sequence
func (h *Handler) TruncateDBTable(c *gin.Context) {
	var input struct {
		Table string `json:"table" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	table := strings.TrimSpace(input.Table)
	if !allowedTables[table] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name"})
		return
	}

	// Delete all rows from table
	if err := h.DB.Exec("DELETE FROM " + table).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Reset sqlite autoincrement sequence if sqlite_sequence exists
	_ = h.DB.Exec("DELETE FROM sqlite_sequence WHERE name = ?", table)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Table '%s' truncated successfully", table)})
}

// BackupDB Downloads a backup copy of adskull.db
func (h *Handler) BackupDB(c *gin.Context) {
	dbPath := "adskull.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database file not found"})
		return
	}

	filename := fmt.Sprintf("adskull_backup_%s.db", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.File(dbPath)
}

// RestoreDB Uploads and replaces adskull.db file safely
func (h *Handler) RestoreDB(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No database file uploaded"})
		return
	}

	dbPath := "adskull.db"
	bakPath := "adskull.db.bak"

	// Backup current database first
	if _, err := os.Stat(dbPath); err == nil {
		src, err := os.Open(dbPath)
		if err == nil {
			dst, err := os.Create(bakPath)
			if err == nil {
				_, _ = io.Copy(dst, src)
				dst.Close()
			}
			src.Close()
		}
	}

	// Save uploaded file to temporary path
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("restore_%d.db", time.Now().UnixNano()))
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save uploaded file: %v", err)})
		return
	}
	defer os.Remove(tmpPath)

	// Copy temporary file to adskull.db
	src, err := os.Open(tmpPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read restored DB file: %v", err)})
		return
	}
	defer src.Close()

	dst, err := os.Create(dbPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to overwrite database file: %v", err)})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write database file: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Database restored successfully! Please restart application to ensure cached states are updated."})
}
