// Package store holds the mutable application state. Gio is immediate-mode, so
// the GUI reads these structs every frame; mutations happen on the main goroutine
// (driven by event processing or channels from background work).
package store

import (
	"sync"

	"github.com/graydovee/todo-manager/desktop/internal/client"
	"github.com/graydovee/todo-manager/desktop/internal/config"
	"github.com/graydovee/todo-manager/desktop/internal/i18n"
)

// Page is the current top-level screen. Only Login and List exist as
// full-screen pages; detail and manage are shown in the side window.
type Page int

const (
	PageLogin Page = iota
	PageList
)

// AppState is the single source of truth for navigation, auth and window mode.
// Access is guarded by mu because background goroutines update fields after
// network calls.
type AppState struct {
	mu sync.Mutex

	Page Page

	// API client; nil until logged in.
	Client *client.Client
	// Persisted config (credentials, window, filters).
	Config *config.Config
	// I18n is the shared translator; SetLang is called when the user switches.
	I18n *i18n.Translator

	// Window mode (top-most / lock). Locking implies top-most.
	TopMost bool
	Locked  bool
	// Dock tracks edge-snapping and auto-hide state.
	Dock DockState

	// SelectedID is the todo currently open in the detail page (0 = none).
	SelectedID uint

	// Status banner / error message, shown briefly.
	Message string
	// Loading overlay (e.g. during connection test).
	Loading bool
}

// NewAppState initialises state from a loaded config.
func NewAppState(cfg *config.Config) *AppState {
	// Resolve the UI language: stored preference, else detect from the system.
	i18n.Default.SetLang(i18n.ParseLang(cfg.Language))
	s := &AppState{
		Config: cfg,
		I18n:   i18n.Default,
		TopMost: cfg.Window.TopMost,
		Locked:  cfg.Window.Locked,
	}
	if cfg.APIKey != "" {
		s.Client = client.New(cfg.BaseURL, cfg.APIKey)
		s.Page = PageList
	} else {
		s.Page = PageLogin
	}
	return s
}

func (s *AppState) Lock()   { s.mu.Lock() }
func (s *AppState) Unlock() { s.mu.Unlock() }

// Login builds a client from base/key and switches to the list page. Caller is
// responsible for persisting config first.
func (s *AppState) Login(baseURL, apiKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config.BaseURL = baseURL
	s.Config.APIKey = apiKey
	s.Client = client.New(baseURL, apiKey)
	s.Page = PageList
}

// Logout clears credentials and returns to the login page.
func (s *AppState) Logout() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config.APIKey = ""
	s.Client = nil
	s.Page = PageLogin
}

// SetMessage sets the transient status banner.
func (s *AppState) SetMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Message = msg
}
