// Package browserboot ensures Chromium required by rod is ready on startup,
// automatically downloads it if not ready, and exposes download progress for the dashboard.
package browserboot

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-rod/rod/lib/launcher"
)

// Status browser readiness / download status snapshot.
type Status struct {
	Ready       bool   `json:"ready"`       // Whether browser is ready for production
	Downloading bool   `json:"downloading"` // Whether downloading is in progress
	Percent     int    `json:"percent"`     // Current phase percentage 0-100
	Phase       string `json:"phase"`       // checking / downloading / unzip / ready / error
	Message     string `json:"message"`     // User-facing message
	Error       string `json:"error"`       // Failure reason
}

// Manager manages browser download status and implements launcher.Browser.Logger to capture progress.
type Manager struct {
	mu   sync.RWMutex
	st   Status
	once sync.Once
}

func New() *Manager {
	return &Manager{st: Status{Phase: "checking", Message: "Checking browser..."}}
}

// Snapshot returns a copy of the status.
func (m *Manager) Snapshot() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.st
}

// Ready returns whether the browser is ready.
func (m *Manager) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.st.Ready
}

func (m *Manager) set(f func(*Status)) {
	m.mu.Lock()
	f(&m.st)
	m.mu.Unlock()
}

// Println implements launcher.Browser.Logger (utils.Logger) interface to parse fetchup progress events.
func (m *Manager) Println(vs ...interface{}) {
	if len(vs) == 0 {
		return
	}
	tag := strings.TrimSpace(fmt.Sprint(vs[0]))
	switch {
	case strings.HasPrefix(tag, "Progress:"):
		if len(vs) > 1 {
			ps := strings.TrimSuffix(strings.TrimSpace(fmt.Sprint(vs[1])), "%")
			if n, err := strconv.Atoi(ps); err == nil {
				m.set(func(s *Status) {
					s.Downloading = true
					if s.Phase != "unzip" {
						s.Phase = "downloading"
					}
					s.Percent = n
					if s.Phase == "unzip" {
						s.Message = fmt.Sprintf("Unzipping browser %d%%", n)
					} else {
						s.Message = fmt.Sprintf("Downloading browser %d%%", n)
					}
				})
			}
		}
	case strings.HasPrefix(tag, "Download:"):
		m.set(func(s *Status) {
			s.Downloading = true
			s.Phase = "downloading"
			s.Percent = 0
			s.Message = "Starting browser download..."
		})
	case strings.HasPrefix(tag, "Unzip:"):
		m.set(func(s *Status) {
			s.Downloading = true
			s.Phase = "unzip"
			s.Percent = 0
			s.Message = "Unzipping browser..."
		})
	case strings.HasPrefix(tag, "Downloaded:"):
		m.set(func(s *Status) {
			s.Percent = 100
			s.Message = "Download complete, verifying..."
		})
	}
}

// EnsureAsync ensures browser readiness in background (idempotent, executes only once).
func (m *Manager) EnsureAsync() {
	m.once.Do(func() { go m.ensure() })
}

func (m *Manager) ensure() {
	b := launcher.NewBrowser()
	b.Logger = m

	// Exists and valid -> ready directly, no download needed.
	if err := b.Validate(); err == nil {
		m.set(func(s *Status) {
			*s = Status{Ready: true, Percent: 100, Phase: "ready", Message: "Browser ready"}
		})
		return
	}

	m.set(func(s *Status) {
		s.Ready = false
		s.Downloading = true
		s.Phase = "downloading"
		s.Message = "Browser missing, downloading..."
	})

	if _, err := b.Get(); err != nil {
		m.set(func(s *Status) {
			s.Ready = false
			s.Downloading = false
			s.Phase = "error"
			s.Error = err.Error()
			s.Message = "Browser download failed, please check network and restart"
		})
		return
	}

	m.set(func(s *Status) {
		*s = Status{Ready: true, Percent: 100, Phase: "ready", Message: "Browser ready"}
	})
}
