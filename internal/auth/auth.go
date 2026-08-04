// Package auth manages admin login and JWT authentication.
//
// Design key points:
//   - Unique token: Each admin has only one valid token at a time; issuing a new one immediately revokes the old.
//   - Persistent token: Database stores the current valid token; memory cache takes priority, re-reads DB if empty after restart.
//   - Auto renewal: Token valid for 24h; after 2h (remaining <22h), requests auto-issue a new token sent via X-New-Token header.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"chatgpt-register/internal/models"
)

const (
	TokenTTL       = 24 * time.Hour
	RenewAfter     = 2 * time.Hour
	DefaultUser    = "admin"
	DefaultPass    = "123123"
	MinPasswordLen = 6
)

var (
	ErrBadCredentials = errors.New("invalid username or password")
	ErrInvalidToken   = errors.New("invalid or expired token")
	ErrWeakPassword   = errors.New("password length must be >= 6 characters")
)

type Claims struct {
	AdminID uint `json:"aid"`
	jwt.RegisteredClaims
}

type cacheEntry struct {
	token    string
	issuedAt time.Time
}

// Service authentication service. tokens is in-memory cache (adminID -> valid token).
// After restart, cache is empty and Validate reads back from DB to populate cache.
type Service struct {
	db     *gorm.DB
	secret []byte

	mu     sync.Mutex
	tokens map[uint]cacheEntry
}

func New(db *gorm.DB) (*Service, error) {
	s := &Service{db: db, tokens: map[uint]cacheEntry{}}
	if err := s.loadSecret(); err != nil {
		return nil, err
	}
	if err := s.ensureAdmin(); err != nil {
		return nil, err
	}
	return s, nil
}

// loadSecret loads JWT signing secret persisted in settings table (randomly generated on first launch).
func (s *Service) loadSecret() error {
	var st models.Setting
	err := s.db.Where("key = ?", "jwt_secret").First(&st).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		st = models.Setting{Key: "jwt_secret", Value: hex.EncodeToString(buf)}
		if err := s.db.Create(&st).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	s.secret = []byte(st.Value)
	return nil
}

// ensureAdmin creates default admin account on first start.
func (s *Service) ensureAdmin() error {
	var count int64
	if err := s.db.Model(&models.Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(DefaultPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Create(&models.Admin{Username: DefaultUser, PasswordHash: string(hash)}).Error
}

// Login validates password and issues a new token (revoking old token).
func (s *Service) Login(username, password string) (string, *models.Admin, error) {
	var a models.Admin
	if err := s.db.Where("username = ?", username).First(&a).Error; err != nil {
		return "", nil, ErrBadCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return "", nil, ErrBadCredentials
	}
	tok, err := s.issueToken(&a)
	if err != nil {
		return "", nil, err
	}
	return tok, &a, nil
}

// issueToken issues a new JWT and writes DB + cache.
func (s *Service) issueToken(a *models.Admin) (string, error) {
	now := time.Now()
	jti := make([]byte, 8)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	claims := Claims{
		AdminID: a.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(jti),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
			Subject:   a.Username,
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", err
	}
	if err := s.db.Model(&models.Admin{}).Where("id = ?", a.ID).
		Updates(map[string]any{"token": tok, "token_issued_at": now}).Error; err != nil {
		return "", err
	}
	s.mu.Lock()
	s.tokens[a.ID] = cacheEntry{token: tok, issuedAt: now}
	s.mu.Unlock()
	return tok, nil
}

// Validate validates token; checks cache first, falls back to DB if cache missed.
// If token issued > 2h, automatically issues new token (newToken non-empty means renewed).
func (s *Service) Validate(tokenStr string) (admin *models.Admin, newToken string, err error) {
	var claims Claims
	t, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil || !t.Valid {
		return nil, "", ErrInvalidToken
	}

	s.mu.Lock()
	entry, cached := s.tokens[claims.AdminID]
	s.mu.Unlock()

	var a models.Admin
	if err := s.db.First(&a, claims.AdminID).Error; err != nil {
		return nil, "", ErrInvalidToken
	}
	if !cached {
		// Cache lost after restart: read back current valid token from database
		entry = cacheEntry{token: a.Token, issuedAt: a.TokenIssuedAt}
		s.mu.Lock()
		s.tokens[a.ID] = entry
		s.mu.Unlock()
	}
	if entry.token == "" || entry.token != tokenStr {
		return nil, "", ErrInvalidToken // Overwritten by new token
	}

	if time.Since(entry.issuedAt) > RenewAfter {
		nt, ierr := s.issueToken(&a)
		if ierr == nil {
			newToken = nt
		}
	}
	return &a, newToken, nil
}

// ChangePassword changes admin password, forcefully issuing a new token upon success.
func (s *Service) ChangePassword(adminID uint, oldPass, newPass string) (string, error) {
	if len(strings.TrimSpace(newPass)) < MinPasswordLen {
		return "", ErrWeakPassword
	}
	var a models.Admin
	if err := s.db.First(&a, adminID).Error; err != nil {
		return "", ErrBadCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(oldPass)) != nil {
		return "", errors.New("incorrect current password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	if err := s.db.Model(&a).Update("password_hash", string(hash)).Error; err != nil {
		return "", err
	}
	return s.issueToken(&a)
}
