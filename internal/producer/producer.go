// Package producer orchestrates batch production of ChatGPT + Codex accounts.
//
// Rules: Each verified mailbox registers 1 single ChatGPT account (using its primary email address).
//
// Target count represents number of successfully produced accounts in this run.
// Failed registrations are auto-retried with new jobs until target is met or mailbox capacity is exhausted.
package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"chatgpt-register/internal/codexreg"
	"chatgpt-register/internal/mailfetch"
	"chatgpt-register/internal/models"

	"gorm.io/gorm"
)

const (
	defaultMaxConcurrency = 10
	codePollTimeout       = 3 * time.Minute
	codePollInterval      = 5 * time.Second
	maxLogLines           = 300
)

// openAI verification code: 6 digits.
var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

// Mailbox sources.
const (
	SourceOutlook  = "outlook"  // Local verified Outlook mailboxes
	SourceVarymail = "varymail" // vary.email fetch: buy per-use mailboxes
)

// Config runtime parameters loaded from settings.
type Config struct {
	MaxConcurrency int
	Headless       bool
	Proxies        []string // Proxy pool, rotated per account; empty = direct

	EmailSource string // outlook / varymail
	VarymailKey string
	LoginOnly   bool
	MailboxIDs  []uint
	Target      int
}

// Progress production progress snapshot for /api/produce/status.
type Progress struct {
	Running    bool      `json:"running"`
	Target     int       `json:"target"`
	Pending    int       `json:"pending"`     // Pending production
	RunningNum int       `json:"running_num"` // Currently running
	Registered int       `json:"registered"`  // Successfully registered
	Failed     int       `json:"failed"`      // Registration failed (cumulative)
	Message    string    `json:"message"`
	Error      string    `json:"error"`
	Logs       []string  `json:"logs"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Producer singleton managing lifecycle and progress of a production task.
type Producer struct {
	db   *gorm.DB
	mail *mailfetch.Client

	mu         sync.Mutex
	prog       Progress
	inflight   map[string]uint // email -> mailboxID
	mbsInUse   map[uint]int    // mailboxID -> active count
	failCount  map[string]int  // email -> consecutive failure count
	failed     map[string]struct{}
	mailboxIDs []uint
	target     int
	pxMu       sync.Mutex
	pxIdx      int

	cancel context.CancelFunc

	claimMu sync.Mutex

	producedMu  sync.Mutex
	producedNum int
}

func New(db *gorm.DB, mail *mailfetch.Client) *Producer {
	return &Producer{
		db:        db,
		mail:      mail,
		inflight:  make(map[string]uint),
		mbsInUse:  make(map[uint]int),
		failCount: make(map[string]int),
		failed:    make(map[string]struct{}),
		prog:      Progress{Logs: make([]string, 0, maxLogLines)},
	}
}

func (p *Producer) getMailboxIDs() []uint {
	p.mu.Lock()
	defer p.mu.Unlock()
	res := make([]uint, len(p.mailboxIDs))
	copy(res, p.mailboxIDs)
	return res
}

func (p *Producer) getTarget() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.target
}

// Start starts a production run (async) or appends mailboxes to an active run.
func (p *Producer) Start(target int, opts ...any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	loginOnly := false
	var mailboxIDs []uint
	for _, opt := range opts {
		switch v := opt.(type) {
		case bool:
			loginOnly = v
		case []uint:
			mailboxIDs = v
		}
	}
	if target < 1 && len(mailboxIDs) == 0 {
		return fmt.Errorf("production target count must be >= 1")
	}
	if len(mailboxIDs) > 0 && target < 1 {
		target = len(mailboxIDs)
	}

	if p.prog.Running {
		for _, id := range mailboxIDs {
			delete(p.failed, fmt.Sprintf("mb_%d", id))
			var mb models.Mailbox
			if err := p.db.Where("id = ?", id).First(&mb).Error; err == nil {
				delete(p.failed, mb.Email)
			}
			p.mailboxIDs = append(p.mailboxIDs, id)
		}
		p.target += target
		p.prog.Target += target
		p.prog.Pending += target
		p.prog.UpdatedAt = time.Now()
		if len(mailboxIDs) > 0 {
			p.logf("➕ Added %d mailbox(es) to running production task queue", len(mailboxIDs))
		} else {
			p.logf("➕ Added +%d target count to running production task queue", target)
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.inflight = map[string]uint{}
	p.mbsInUse = map[uint]int{}
	p.failCount = map[string]int{}
	p.failed = map[string]struct{}{}
	p.mailboxIDs = mailboxIDs
	p.target = target
	p.producedNum = 0
	p.prog = Progress{Running: true, Target: target, Pending: target, Message: "Initializing...", UpdatedAt: time.Now()}
	go p.run(ctx, loginOnly)
	return nil
}

func (p *Producer) run(ctx context.Context, loginOnly bool) {
	defer func() {
		p.mu.Lock()
		p.prog.Running = false
		p.recalcLocked()
		p.prog.UpdatedAt = time.Now()
		p.mu.Unlock()
	}()

	cfg := p.loadConfig()
	cfg.LoginOnly = loginOnly
	if cfg.EmailSource == SourceVarymail {
		p.runVarymail(ctx, p.getTarget(), cfg)
		return
	}
	p.logf("Starting production: target %d accounts (concurrency %d)", p.getTarget(), cfg.MaxConcurrency)

	sem := make(chan struct{}, cfg.MaxConcurrency)
	var wg sync.WaitGroup

	for {
		if ctx.Err() != nil {
			p.logf("Stopped manually")
			break
		}
		target := p.getTarget()
		done := p.producedThisRun()
		running := p.inflightCount()
		if done+running >= target {
			if running == 0 {
				break
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		mb, email, ok := p.nextJob(cfg)
		if !ok {
			if p.inflightCount() == 0 {
				p.logf("No more mailbox capacity available, produced %d in this run", p.producedThisRun())
				break
			}
			time.Sleep(800 * time.Millisecond)
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(mb models.Mailbox, email string) {
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

			if err := p.produceOne(ctx, cfg, mb, email); err != nil {
				if errors.Is(err, codexreg.ErrAccountTaken) {
					p.logf("⚠ %s disabled (%v), skipping to next address", mask(email), err)
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
		}(mb, email)
	}

	wg.Wait()
	produced := p.producedThisRun()
	if ctx.Err() != nil {
		p.setMessage(fmt.Sprintf("Stopped, successfully produced %d", produced))
	} else {
		p.setMessage(fmt.Sprintf("Completed, successfully produced %d", produced))
	}
}

// Stop requests stopping production (currently running tasks will finish).
func (p *Producer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
	}
}

// Snapshot returns progress copy.
func (p *Producer) Snapshot() Progress {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := p.prog
	cp.Logs = append([]string(nil), p.prog.Logs...)
	return cp
}

// nextJob claims next account to register: backfills 2FA for registered accounts missing 2FA first, then registers un-registered mailboxes.
func (p *Producer) nextJob(cfg Config) (models.Mailbox, string, bool) {
	p.claimMu.Lock()
	defer p.claimMu.Unlock()

	mailboxIDs := p.getMailboxIDs()
	target := p.getTarget()

	// In LoginOnly mode: convert verified mailboxes without a Registration entry into status = 'registered' (missing 2FA/password)
	if cfg.LoginOnly {
		var verifiedMbs []models.Mailbox
		q := p.db.Where("status = ? AND provider <> ?", "verified", SourceVarymail)
		if len(mailboxIDs) > 0 {
			q = q.Where("id IN ?", mailboxIDs)
		}
		if err := q.Order("id asc").Find(&verifiedMbs).Error; err == nil {
			createdCount := 0
			for _, mb := range verifiedMbs {
				var count int64
				p.db.Model(&models.Registration{}).Where("email = ? OR mailbox_id = ?", mb.Email, mb.ID).Count(&count)
				if count == 0 {
					var currentUnset2FA int64
					p.db.Model(&models.Registration{}).Where("status = ? AND (two_factor_secret IS NULL OR two_factor_secret = '')", "registered").Count(&currentUnset2FA)
					if target > 0 && int(currentUnset2FA) >= target {
						break
					}
					p.db.Create(&models.Registration{
						Email:     mb.Email,
						MailboxID: mb.ID,
						Password:  mb.Password,
						Status:    "registered",
						IsMother:  true,
						Note:      "login_only",
					})
					createdCount++
					if target > 0 && createdCount >= target {
						break
					}
				}
			}
		}
	}

	// Pass 0: Registered accounts missing 2FA secret -> auto login and set 2FA (only when specific mailboxes requested or in LoginOnly mode)
	if len(mailboxIDs) > 0 || cfg.LoginOnly {
		var missing2FA []models.Registration
		q0 := p.db.Where("status = ? AND (two_factor_secret IS NULL OR two_factor_secret = '')", "registered")
		if len(mailboxIDs) > 0 {
			q0 = q0.Where("mailbox_id IN ?", mailboxIDs)
		}
		if err := q0.Order("id asc").Find(&missing2FA).Error; err == nil {
			for _, reg := range missing2FA {
				p.mu.Lock()
				_, busy := p.inflight[reg.Email]
				p.mu.Unlock()
				if busy {
					continue
				}

				var mb models.Mailbox
				if err := p.db.Where("id = ?", reg.MailboxID).First(&mb).Error; err != nil {
					mb = models.Mailbox{ID: reg.MailboxID, Email: reg.Email, Password: reg.Password}
				}
				if mb.ID > 0 && p.mailboxBusy(mb.ID) {
					continue
				}

				p.markInflight(reg.Email, mb.ID)
				return mb, reg.Email, true
			}
		}
	}

	if cfg.LoginOnly {
		return models.Mailbox{}, "", false
	}

	var mailboxes []models.Mailbox
	qMB := p.db.Where("status = ? AND provider <> ?", "verified", SourceVarymail)
	if len(mailboxIDs) > 0 {
		qMB = qMB.Where("id IN ?", mailboxIDs)
	}
	if err := qMB.Order("id asc").Find(&mailboxes).Error; err != nil {
		return models.Mailbox{}, "", false
	}

	// Pass 1: Mailbox not registered and mailbox free -> register account
	for _, mb := range mailboxes {
		if p.mailboxBusy(mb.ID) {
			continue
		}
		if len(mailboxIDs) > 0 || !p.isRegistered(mb.Email, mb.ID) {
			p.markInflight(mb.Email, mb.ID)
			return mb, mb.Email, true
		}
	}
	return models.Mailbox{}, "", false
}

// produceOne produces a single account: Register ChatGPT -> Generate Codex agent identity -> Store DB.
func (p *Producer) produceOne(ctx context.Context, cfg Config, mb models.Mailbox, email string) error {
	var existing models.Registration
	_ = p.db.Where("email = ?", email).First(&existing).Error

	// If account already has BOTH password AND 2FA set up, skip it completely
	if (existing.Status == "registered" || existing.Status == "already_registered") && existing.Password != "" && existing.TwoFactorSecret != "" {
		p.logf("ℹ️ %s already has password and 2FA configured, skipping", mask(email))
		return nil
	}

	password := codexreg.GenPassword(16)
	if existing.Password != "" {
		password = existing.Password
	}

	note := ""
	isLoginOnly := cfg.LoginOnly || existing.Note == "login_only" || (existing.Status == "registered" && (existing.TwoFactorSecret == "" || existing.Password == ""))
	status := "registering"
	if isLoginOnly {
		status = "logging_in"
	}

	p.upsert(models.Registration{
		Email: email, MailboxID: mb.ID, Password: password,
		Status: status, IsMother: true, Note: note,
	})

	var logMu sync.Mutex
	var logBuf strings.Builder
	if strings.TrimSpace(existing.Log) != "" {
		logBuf.WriteString(existing.Log)
		if !strings.HasSuffix(existing.Log, "\n") {
			logBuf.WriteString("\n")
		}
		if isLoginOnly {
			logBuf.WriteString("[" + time.Now().Format("02/01/2006 15:04:05") + "] --- New login & 2FA setup attempt ---\n")
		} else {
			logBuf.WriteString("[" + time.Now().Format("02/01/2006 15:04:05") + "] --- New registration attempt ---\n")
		}
	}
	appendLog := func(line string) {
		logMu.Lock()
		logBuf.WriteString("[" + time.Now().Format("02/01/2006 15:04:05") + "] " + line + "\n")
		snapshot := logBuf.String()
		logMu.Unlock()
		p.db.Model(&models.Registration{}).Where("email = ?", email).Update("log", snapshot)
	}

	since := time.Now().Add(-30 * time.Second)
	in := codexreg.Input{
		Email:           email,
		Password:        password,
		TwoFactorSecret: existing.TwoFactorSecret,
		Proxy:           p.nextProxy(cfg),
		Headless:        cfg.Headless,
		IsLoginOnly:     isLoginOnly,
		Log: func(f string, a ...any) {
			msg := fmt.Sprintf(f, a...)
			appendLog(msg)
			p.logf("%s", "  "+mask(email)+" "+msg)
		},
		FetchCode: func(ctx context.Context) (string, error) {
			return p.fetchCode(ctx, mb, since)
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

	if isLoginOnly {
		appendLog("✓ Login & 2FA setup success")
	} else {
		appendLog("✓ Registration success")
	}
	authBytes, _ := json.MarshalIndent(res.AuthJSON, "", "  ")
	finalPassword := password
	if res.Password != "" {
		finalPassword = res.Password
	}
	p.upsert(models.Registration{
		Email: email, MailboxID: mb.ID, Password: finalPassword,
		Status: "registered", IsMother: true, Note: note,
		AuthData: string(authBytes), AccountID: res.AccountID,
		UserID: res.UserID, PlanType: res.PlanType,
		TwoFactorSecret: res.TwoFactorSecret, Log: logBuf.String(),
	})
	if mb.ID > 0 {
		updates := map[string]any{}
		if res.TwoFactorSecret != "" {
			updates["two_factor_secret"] = res.TwoFactorSecret
		}
		if finalPassword != "" {
			updates["password"] = finalPassword
		}
		if len(updates) > 0 {
			p.db.Model(&models.Mailbox{}).Where("id = ?", mb.ID).Updates(updates)
		}
	}
	return nil
}

// fetchCode polls mailbox for 6-digit verification code from OpenAI/ChatGPT emails or 2FA secret.
func (p *Producer) fetchCode(ctx context.Context, mb models.Mailbox, since time.Time) (string, error) {
	acc := mailfetch.Account{Email: mb.Email, ClientID: mb.ClientID, RefreshToken: mb.RefreshToken}
	deadline := time.Now().Add(codePollTimeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		msgs, err := p.mail.ListMessages(ctx, acc, 15)
		if err == nil {
			for _, m := range msgs {
				if m.ReceivedAt.Before(since) || !looksLikeOpenAI(m) {
					continue
				}
				if code := codeRe.FindStringSubmatch(m.Subject); code != nil {
					return code[1], nil
				}
				full, gerr := p.mail.GetMessage(ctx, acc, m.ID)
				if gerr != nil {
					continue
				}
				if code := codeRe.FindStringSubmatch(full.Subject + " " + full.Text); code != nil {
					return code[1], nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(codePollInterval):
		}
	}
	return "", fmt.Errorf("timeout waiting for verification code email")
}

func looksLikeOpenAI(m mailfetch.Message) bool {
	s := strings.ToLower(m.From + " " + m.FromName + " " + m.Subject)
	return strings.Contains(s, "openai") || strings.Contains(s, "chatgpt") || strings.Contains(s, "code")
}

// ---- inflight / counts ----

func (p *Producer) markInflight(email string, mbID uint) {
	p.mu.Lock()
	p.inflight[email] = mbID
	p.mu.Unlock()
}

func (p *Producer) releaseInflight(email string) {
	p.mu.Lock()
	delete(p.inflight, email)
	p.mu.Unlock()
}

func (p *Producer) inflightCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.inflight)
}

// mailboxBusy whether mailbox has active task running.
func (p *Producer) mailboxBusy(mbID uint) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range p.inflight {
		if id == mbID {
			return true
		}
	}
	return false
}

// counts returns (registeredCount, runningCount).
func (p *Producer) counts() (int, int) {
	registered := p.registeredCount()
	return registered, p.inflightCount()
}

// producedThisRun returns count of successfully produced accounts in current run.
func (p *Producer) producedThisRun() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prog.Registered
}

func (p *Producer) registeredCount() int {
	var n int64
	p.db.Model(&models.Registration{}).Where("status = ?", "registered").Count(&n)
	return int(n)
}

// isRegistered returns true if address or mailbox already has a registration with status 'registered' or 'already_registered'.
func (p *Producer) isRegistered(email string, mbID ...uint) bool {
	var n int64
	q := p.db.Model(&models.Registration{}).Where("status IN ?", []string{"registered", "already_registered"})
	if len(mbID) > 0 && mbID[0] > 0 {
		q = q.Where("mailbox_id = ? OR email = ?", mbID[0], email)
	} else {
		q = q.Where("email = ?", email)
	}
	q.Count(&n)
	return n > 0
}

// ---- DB / Settings ----

func (p *Producer) loadConfig() Config {
	cfg := Config{
		MaxConcurrency: atoiDefault(p.getSetting("max_concurrency"), defaultMaxConcurrency),
		Headless:       p.getSetting("headless") != "0",
	}
	if cfg.MaxConcurrency < 1 {
		cfg.MaxConcurrency = 1
	}
	if p.getSetting("proxy_enabled") == "1" {
		cfg.Proxies = proxyList(p.getSetting("proxy_list"))
	}

	cfg.EmailSource = p.getSetting("email_source")
	if cfg.EmailSource != SourceVarymail {
		cfg.EmailSource = SourceOutlook
	}
	cfg.VarymailKey = p.getSetting("varymail_api_key")
	return cfg
}

// nextProxy fetches next proxy from pool round-robin style; returns empty string if pool empty.
func (p *Producer) nextProxy(cfg Config) string {
	if len(cfg.Proxies) == 0 {
		return ""
	}
	p.pxMu.Lock()
	proxy := cfg.Proxies[p.pxIdx%len(cfg.Proxies)]
	p.pxIdx++
	p.pxMu.Unlock()
	return proxy
}

func (p *Producer) upsert(reg models.Registration) {
	var existing models.Registration
	if err := p.db.Where("email = ?", reg.Email).First(&existing).Error; err == nil {
		updates := map[string]any{
			"password": reg.Password, "status": reg.Status,
			"is_mother": reg.IsMother, "note": reg.Note, "mailbox_id": reg.MailboxID,
		}
		if reg.TwoFactorSecret != "" {
			updates["two_factor_secret"] = reg.TwoFactorSecret
		}
		if reg.AuthData != "" {
			updates["auth_data"] = reg.AuthData
			updates["account_id"] = reg.AccountID
			updates["user_id"] = reg.UserID
			updates["plan_type"] = reg.PlanType
		}
		if reg.Log != "" {
			updates["log"] = reg.Log
		}
		p.db.Model(&existing).Updates(updates)
		return
	}
	p.db.Create(&reg)
}

func (p *Producer) setRegistrationFailed(email, note, log string) {
	p.setRegistrationStatus(email, "register_failed", note, log)
}

func (p *Producer) setRegistrationStatus(email, status, note, log string) {
	upd := map[string]any{"status": status, "note": truncateStr(note, 500)}
	if log != "" {
		upd["log"] = log
	}
	p.db.Model(&models.Registration{}).Where("email = ?", email).Updates(upd)
}

func (p *Producer) getSetting(key string) string {
	var items []models.Setting
	if err := p.db.Where("key = ?", key).Limit(1).Find(&items).Error; err != nil || len(items) == 0 {
		return ""
	}
	return items[0].Value
}

// ---- Progress ----

func (p *Producer) logf(format string, a ...any) {
	line := "[" + time.Now().Format("02/01/2006 15:04:05") + "] " + fmt.Sprintf(format, a...)
	p.mu.Lock()
	p.prog.Logs = append(p.prog.Logs, line)
	if len(p.prog.Logs) > maxLogLines {
		p.prog.Logs = p.prog.Logs[len(p.prog.Logs)-maxLogLines:]
	}
	p.prog.UpdatedAt = time.Now()
	p.mu.Unlock()
}

func (p *Producer) incRegistered() {
	p.mu.Lock()
	p.prog.Registered++
	p.recalcLocked()
	p.mu.Unlock()
}

// markFailed marks mailbox in failed state.
func (p *Producer) markFailed(email string) {
	p.mu.Lock()
	p.failed[email] = struct{}{}
	p.prog.Failed = len(p.failed)
	p.mu.Unlock()
}

// markSuccess removes mailbox from failed state upon success.
func (p *Producer) markSuccess(email string) {
	p.mu.Lock()
	delete(p.failed, email)
	p.prog.Failed = len(p.failed)
	p.mu.Unlock()
}

func (p *Producer) updateProgress() {
	p.mu.Lock()
	p.recalcLocked()
	p.mu.Unlock()
}

// recalcLocked recalculates pending and running numbers. Caller must hold lock.
func (p *Producer) recalcLocked() {
	p.prog.RunningNum = len(p.inflight)
	pending := p.prog.Target - p.prog.Registered - p.prog.RunningNum
	if pending < 0 {
		pending = 0
	}
	p.prog.Pending = pending
	p.prog.UpdatedAt = time.Now()
}

func (p *Producer) setMessage(msg string) {
	p.mu.Lock()
	p.prog.Message = msg
	p.prog.UpdatedAt = time.Now()
	p.mu.Unlock()
}
