// Package replenish Auto-replenish: Periodically pushes local registered ChatGPT accounts (access_token)
// to image2api account pool. Every 30 seconds queries target site alive openai account count,
// and if below threshold, replenishes difference using unshipped registered accounts.
package replenish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"chatgpt-register/internal/browserboot"
	"chatgpt-register/internal/models"
	"chatgpt-register/internal/producer"

	"gorm.io/gorm"
)

// maxBatch Maximum push per single round.
const maxBatch = 50

// pushMin Minimum count before pushing.
const pushMin = 5

// Service Auto-replenish pusher.
type Service struct {
	db      *gorm.DB
	prod    *producer.Producer
	browser *browserboot.Manager
	client  *http.Client
	busy    atomic.Bool

	mu        sync.Mutex
	token     string    // Cached image2api session token
	tokenAt   time.Time // Token login time
	nextLogin time.Time // Cooldown end time after login failure
}

// New constructs replenish service.
func New(db *gorm.DB, prod *producer.Producer, browser *browserboot.Manager) *Service {
	return &Service{
		db:      db,
		prod:    prod,
		browser: browser,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Start starts background loop, replenishing every 30 seconds until ctx cancelled.
func (s *Service) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

// tick Single round replenish.
func (s *Service) tick(ctx context.Context) {
	if !s.busy.CompareAndSwap(false, true) {
		return
	}
	defer s.busy.Store(false)

	cfg := s.settings()
	if cfg["replenish_enabled"] != "1" {
		return
	}
	threshold, _ := strconv.Atoi(strings.TrimSpace(cfg["replenish_threshold"]))
	base := strings.TrimRight(strings.TrimSpace(cfg["replenish_target_url"]), "/")
	user := strings.TrimSpace(cfg["replenish_username"])
	pass := cfg["replenish_password"]
	if threshold <= 0 || base == "" || user == "" || pass == "" {
		return
	}

	alive, err := s.aliveCount(ctx, base, user, pass)
	if err != nil {
		log.Printf("Replenish: query target alive count failed: %v", err)
		return
	}
	need := threshold - alive
	if need <= 0 {
		return
	}

	var ready int64
	s.db.Model(&models.Registration{}).
		Where("status = ? AND shipped = ? AND auth_data <> ''", "registered", false).
		Count(&ready)

	moreComing := s.prod != nil && s.prod.Snapshot().Running
	if cfg["replenish_autoproduce"] == "1" && s.prod != nil {
		if want := need - int(ready); want > 0 && !moreComing {
			if s.browser != nil && s.browser.Ready() {
				if err := s.prod.Start(want); err != nil {
					log.Printf("Replenish: auto production failed to start: %v", err)
				} else {
					moreComing = true
					log.Printf("Replenish: target alive %d < threshold %d, ready %d insufficient, auto producing %d", alive, threshold, ready, want)
				}
			} else {
				log.Printf("Replenish: auto production needed but browser not ready, skipping round")
			}
		}
	}

	pushN := need
	if int(ready) < pushN {
		pushN = int(ready)
	}
	if pushN < pushMin && moreComing {
		return
	}
	if pushN <= 0 {
		return
	}
	if pushN > maxBatch {
		pushN = maxBatch
	}

	var regs []models.Registration
	if err := s.db.WithContext(ctx).
		Where("status = ? AND shipped = ? AND auth_data <> ''", "registered", false).
		Order("id asc").Limit(pushN).Find(&regs).Error; err != nil {
		log.Printf("Replenish: query ready accounts failed: %v", err)
		return
	}

	pushed := 0
	for _, r := range regs {
		tok := accessToken(r.AuthData)
		if tok == "" {
			continue
		}
		if err := s.importToken(ctx, base, user, pass, tok); err != nil {
			log.Printf("Replenish: pushing account #%d failed: %v", r.ID, err)
			continue
		}
		if err := s.db.Model(&models.Registration{}).Where("id = ?", r.ID).Update("shipped", true).Error; err != nil {
			log.Printf("Replenish: mark #%d shipped failed: %v", r.ID, err)
		}
		pushed++
	}
	if pushed > 0 {
		log.Printf("Replenish: target alive %d < threshold %d, pushed %d accounts", alive, threshold, pushed)
	}
}

// settings reads all settings into map.
func (s *Service) settings() map[string]string {
	var items []models.Setting
	if err := s.db.Find(&items).Error; err != nil {
		log.Printf("Replenish: failed to read settings: %v", err)
		return map[string]string{}
	}
	m := make(map[string]string, len(items))
	for _, it := range items {
		m[it.Key] = it.Value
	}
	return m
}

// aliveCount queries image2api alive openai account count.
func (s *Service) aliveCount(ctx context.Context, base, user, pass string) (int, error) {
	resp, err := s.authedRequest(ctx, base, user, pass, http.MethodGet, "/admin/api/accounts", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Stats struct {
			Openai struct {
				N     int `json:"n"`
				Ok    int `json:"ok"`
				Dead  int `json:"dead"`
				Quota int `json:"quota"`
			} `json:"openai"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}
	st := out.Stats.Openai
	alive := st.N - st.Dead - st.Quota
	if alive < 0 {
		alive = 0
	}
	return alive, nil
}

// importToken imports an access_token into image2api chatgpt pool.
func (s *Service) importToken(ctx context.Context, base, user, pass, token string) error {
	body, _ := json.Marshal(map[string]string{"access_token": token})
	resp, err := s.authedRequest(ctx, base, user, pass, http.MethodPost, "/admin/api/tokens/import-chatgpt-token", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

// authedRequest makes authenticated HTTP request.
func (s *Service) authedRequest(ctx context.Context, base, user, pass, method, path string, body []byte) (*http.Response, error) {
	send := func(tok string) (*http.Response, error) {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, base+path, rdr)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		return s.client.Do(req)
	}

	tok, err := s.ensureToken(ctx, base, user, pass)
	if err != nil {
		return nil, err
	}
	resp, err := send(tok)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		s.mu.Lock()
		s.token = ""
		s.mu.Unlock()
		if tok, err = s.ensureToken(ctx, base, user, pass); err != nil {
			return nil, err
		}
		return send(tok)
	}
	return resp, nil
}

// ensureToken returns a valid session token.
func (s *Service) ensureToken(ctx context.Context, base, user, pass string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Since(s.tokenAt) < time.Hour {
		return s.token, nil
	}
	if !s.nextLogin.IsZero() && time.Now().Before(s.nextLogin) {
		return "", fmt.Errorf("login in cooldown, please retry later")
	}
	tok, err := s.login(ctx, base, user, pass)
	if err != nil {
		s.nextLogin = time.Now().Add(5 * time.Minute)
		return "", err
	}
	s.nextLogin = time.Time{}
	s.token = tok
	s.tokenAt = time.Now()
	return tok, nil
}

// login logs into image2api using credentials from settings.
func (s *Service) login(ctx context.Context, base, user, pass string) (string, error) {
	body, _ := json.Marshal(map[string]string{"identifier": user, "password": pass})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/admin/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.Token == "" {
		return "", fmt.Errorf("login response missing token")
	}
	return out.Token, nil
}

// accessToken extracts access_token from stored auth.json in DB.
func accessToken(authData string) string {
	var parsed map[string]any
	_ = json.Unmarshal([]byte(authData), &parsed)
	s, _ := parsed["access_token"].(string)
	return s
}
