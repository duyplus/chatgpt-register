package db

import (
	"chatgpt-register/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func Init(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.Registration{}, &models.Mailbox{}, &models.Setting{}, &models.Admin{}); err != nil {
		return nil, err
	}
	normalizeLegacyStatuses(db)
	reclaimOrphanRegistering(db)
	backfillRegistrationMailboxIDs(db)
	dropMailboxTypeColumn(db)
	dropMailboxTwoFactorSecretColumn(db)
	return db, nil
}

func dropMailboxTypeColumn(db *gorm.DB) {
	if db.Migrator().HasColumn(&models.Mailbox{}, "type") {
		_ = db.Migrator().DropColumn(&models.Mailbox{}, "type")
	}
}

func dropMailboxTwoFactorSecretColumn(db *gorm.DB) {
	if db.Migrator().HasColumn(&models.Mailbox{}, "two_factor_secret") {
		_ = db.Migrator().DropColumn(&models.Mailbox{}, "two_factor_secret")
	}
}

// reclaimOrphanRegistering marks leftover registering records as register_failed on startup.
// Production task status is in-memory; after restart, these "registering" records won't be pushed further.
// Setting them to failed allows them to be reclaimed during the next production run.
func reclaimOrphanRegistering(db *gorm.DB) {
	db.Model(&models.Registration{}).Where("status = ?", "registering").
		Updates(map[string]any{"status": "register_failed", "note": "Interrupted by program restart, can be reproduced"})
}

// normalizeLegacyStatuses migrates legacy verification status registration records to new production status.
// Mailbox's unverified/verified indicates whether mailbox credentials are valid, semantics unchanged.
func normalizeLegacyStatuses(db *gorm.DB) {
	regStatusMap := map[string]string{
		"unverified":    "pending",
		"verifying":     "registering",
		"verify_failed": "register_failed",
		"verified":      "registered",
	}
	for oldStatus, newStatus := range regStatusMap {
		db.Model(&models.Registration{}).Where("status = ?", oldStatus).Update("status", newStatus)
	}
}

func backfillRegistrationMailboxIDs(db *gorm.DB) {
	var regs []models.Registration
	if err := db.Where("mailbox_id IS NULL OR mailbox_id = 0").Find(&regs).Error; err != nil {
		return
	}
	for _, reg := range regs {
		var mb models.Mailbox
		if err := db.Where("email = ?", reg.Email).First(&mb).Error; err == nil {
			db.Model(&models.Registration{}).Where("id = ?", reg.ID).Update("mailbox_id", mb.ID)
		}
	}
}
