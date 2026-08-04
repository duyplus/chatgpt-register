package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"chatgpt-register/internal/codexreg"
	"chatgpt-register/internal/models"
	"chatgpt-register/internal/varymail"
)

// runVarymail produces accounts using vary.email fetch service as mailbox source.
func (p *Producer) runVarymail(ctx context.Context, target int, cfg Config) {
	if strings.TrimSpace(cfg.VarymailKey) == "" {
		p.setMessage("varymail API Key not configured, cannot produce")
		p.logf("✗ varymail unconfigured: please fill in API Key in settings")
		return
	}
	cli := varymail.New("", cfg.VarymailKey)

	svc, _, err := cli.ServiceByName(ctx, varymail.DefaultServiceName)
	if err != nil {
		p.setMessage("varymail connection failed: " + err.Error())
		p.logf("✗ varymail service check failed: %v", err)
		return
	}
	p.logf("Starting production (varymail), target %d, service=%s stock=%s available=%d concurrency %d",
		target, svc.Name, svc.Stock, svc.Available, cfg.MaxConcurrency)

	sem := make(chan struct{}, cfg.MaxConcurrency)
	var wg sync.WaitGroup
	var haltMsg string

	startJob := func(mb models.Mailbox, email string) {
		sem <- struct{}{}
		wg.Add(1)
		go func(email string, mailboxID uint, purchaseID int) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				p.releaseInflight(email)
				p.updateProgress()
			}()
			defer func() {
				if r := recover(); r != nil {
					p.markFailed(email)
					msg := fmt.Sprintf("Registration panic: %v", r)
					p.setRegistrationFailed(email, msg, "")
					p.logf("✗ %s %s\n%s", mask(email), msg, debug.Stack())
					p.updateProgress()
				}
			}()
			p.updateProgress()

			if err := p.produceOneVarymail(ctx, cfg, cli, email, mailboxID, purchaseID); err != nil {
				if errors.Is(err, codexreg.ErrAccountTaken) {
					p.logf("⚠ %s disabled (%v), skipping", mask(email), err)
				} else {
					p.markFailed(email)
					p.logf("✗ %s registration failed: %v", mask(email), err)
				}
			} else {
				p.markSuccess(email)
				p.incRegistered()
				p.logf("✓ %s registration success", mask(email))
			}
			p.updateProgress()
		}(email, mb.ID, mb.PurchaseID)
	}

	for {
		if ctx.Err() != nil {
			p.logf("Stopped manually")
			break
		}
		done := p.producedThisRun()
		running := p.inflightCount()
		if done+running >= target {
			if running == 0 {
				break
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if mb, email, ok := p.claimVarymailJob(cfg); ok {
			if _, _, cerr := cli.Code(ctx, mb.PurchaseID); errors.Is(cerr, varymail.ErrPickup) {
				p.db.Model(&models.Mailbox{}).Where("id = ?", mb.ID).
					Updates(map[string]any{"status": "verify_failed", "note": "Pickup expired"})
				p.releaseInflight(email)
				p.logf("⚠ %s pickup expired, removed from pool", mask(mb.Email))
				continue
			}
			p.logf("♻ Reusing purchased mailbox %s", mask(email))
			startJob(mb, email)
			continue
		}

		if svc.Stock == "out" || svc.Available <= 0 {
			p.setMessage(fmt.Sprintf("varymail service '%s' out of stock", svc.Name))
			p.logf("✗ varymail out of stock (%s), stopping purchase", svc.Name)
			if p.inflightCount() == 0 {
				break
			}
			time.Sleep(800 * time.Millisecond)
			continue
		}

		pur, bal, err := cli.Buy(ctx, svc.ID)
		if err != nil {
			switch {
			case errors.Is(err, varymail.ErrOutOfStock):
				p.logf("⚠ varymail out of stock, stopping new tasks")
				haltMsg = "varymail out of stock, stopped"
			case errors.Is(err, varymail.ErrNoBalance):
				p.logf("✗ varymail insufficient balance, stopping production")
				haltMsg = "varymail insufficient balance, please top up"
			case errors.Is(err, varymail.ErrUnauthorized):
				p.logf("✗ varymail API Key invalid, stopping production")
				haltMsg = "varymail API Key invalid"
			default:
				p.logf("✗ varymail order failed: %v", err)
				haltMsg = "varymail order failed: " + err.Error()
			}
			if p.inflightCount() == 0 {
				break
			}
			time.Sleep(800 * time.Millisecond)
			continue
		}

		email := strings.TrimSpace(pur.Email)
		if email == "" {
			p.logf("✗ varymail order did not return email, skipping")
			continue
		}
		mb := p.saveVarymailMailbox(email, pur.ID)
		p.markInflight(email, mb.ID)
		p.logf("🛒 varymail allocated mailbox %s (balance %.2f)", mask(email), bal)

		startJob(mb, email)
	}

	wg.Wait()
	produced := p.producedThisRun()
	switch {
	case ctx.Err() != nil:
		p.setMessage(fmt.Sprintf("Stopped, successfully produced %d", produced))
	case haltMsg != "":
		p.setMessage(fmt.Sprintf("%s (successfully produced %d)", haltMsg, produced))
	default:
		p.setMessage(fmt.Sprintf("Completed, successfully produced %d", produced))
	}
}

func (p *Producer) claimVarymailJob(cfg Config) (models.Mailbox, string, bool) {
	p.claimMu.Lock()
	defer p.claimMu.Unlock()

	var boxes []models.Mailbox
	if err := p.db.Where("provider = ? AND status = ?", SourceVarymail, "verified").
		Order("id asc").Find(&boxes).Error; err != nil {
		return models.Mailbox{}, "", false
	}

	for _, mb := range boxes {
		if mb.PurchaseID <= 0 || p.mailboxBusy(mb.ID) {
			continue
		}
		if !p.isRegistered(mb.Email) {
			p.markInflight(mb.Email, mb.ID)
			return mb, mb.Email, true
		}
	}
	return models.Mailbox{}, "", false
}

func (p *Producer) saveVarymailMailbox(email string, purchaseID int) models.Mailbox {
	var mb models.Mailbox
	if err := p.db.Where("email = ?", email).First(&mb).Error; err == nil {
		p.db.Model(&mb).Updates(map[string]any{
			"provider": SourceVarymail, "purchase_id": purchaseID, "status": "verified",
		})
		mb.Provider = SourceVarymail
		mb.PurchaseID = purchaseID
		mb.Status = "verified"
		return mb
	}
	mb = models.Mailbox{
		Email: email, Provider: SourceVarymail, PurchaseID: purchaseID,
		Status: "verified", Note: "vary.email purchase",
	}
	p.db.Create(&mb)
	return mb
}

func (p *Producer) produceOneVarymail(ctx context.Context, cfg Config, cli *varymail.Client, email string, mailboxID uint, purchaseID int) error {
	password := codexreg.GenPassword(16)
	note := "varymail"
	p.upsert(models.Registration{
		Email: email, MailboxID: mailboxID, Password: password,
		Status: "registering", IsMother: true, Note: note,
	})

	var logMu sync.Mutex
	var logBuf strings.Builder
	var existing models.Registration
	if err := p.db.Select("log").Where("email = ?", email).First(&existing).Error; err == nil && strings.TrimSpace(existing.Log) != "" {
		logBuf.WriteString(existing.Log)
		if !strings.HasSuffix(existing.Log, "\n") {
			logBuf.WriteString("\n")
		}
		logBuf.WriteString("[" + time.Now().Format("02/01/2006 15:04:05") + "] --- New registration attempt ---\n")
	}
	appendLog := func(line string) {
		logMu.Lock()
		logBuf.WriteString("[" + time.Now().Format("02/01/2006 15:04:05") + "] " + line + "\n")
		snapshot := logBuf.String()
		logMu.Unlock()
		p.db.Model(&models.Registration{}).Where("email = ?", email).Update("log", snapshot)
	}

	baselineID := ""
	if msg, hasMail, err := cli.Code(ctx, purchaseID); err == nil && hasMail {
		baselineID = msg.ID
	}

	in := codexreg.Input{
		Email:    email,
		Password: password,
		Proxy:    p.nextProxy(cfg),
		Headless: cfg.Headless,
		Log: func(f string, a ...any) {
			msg := fmt.Sprintf(f, a...)
			appendLog(msg)
			p.logf("%s", "  "+mask(email)+" "+msg)
		},
		FetchCode: func(ctx context.Context) (string, error) {
			return p.fetchCodeVarymail(ctx, cli, purchaseID, baselineID)
		},
		SaveShot: func(png []byte) {
			p.db.Model(&models.Registration{}).Where("email = ?", email).Update("shot", png)
		},
	}

	res, err := codexreg.Register(ctx, in)
	if err != nil {
		if errors.Is(err, codexreg.ErrAccountTaken) {
			appendLog("🛑 Account deactivated/banned by OpenAI (Authentication Error), stopping execution and setting status to Banned")
			p.setRegistrationStatus(email, "already_registered", "Banned: "+err.Error(), logBuf.String())
			return err
		}
		appendLog("✗ Failed: " + err.Error())
		p.setRegistrationFailed(email, err.Error(), logBuf.String())
		return err
	}

	appendLog("✓ Registration success")
	authBytes, _ := json.MarshalIndent(res.AuthJSON, "", "  ")
	p.upsert(models.Registration{
		Email: email, MailboxID: mailboxID, Password: password,
		Status: "registered", IsMother: true, Note: note,
		AuthData: string(authBytes), AccountID: res.AccountID,
		UserID: res.UserID, PlanType: res.PlanType, Log: logBuf.String(),
	})
	return nil
}

func (p *Producer) fetchCodeVarymail(ctx context.Context, cli *varymail.Client, purchaseID int, ignoreID string) (string, error) {
	deadline := time.Now().Add(codePollTimeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		msg, hasMail, err := cli.Code(ctx, purchaseID)
		switch {
		case errors.Is(err, varymail.ErrPickup):
		case err != nil:
			return "", err
		case hasMail && msg.ID != ignoreID && strings.TrimSpace(msg.Code) != "":
			return strings.TrimSpace(msg.Code), nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(codePollInterval):
		}
	}
	return "", fmt.Errorf("timeout waiting for verification code")
}
