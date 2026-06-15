package session

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/graydovee/todo-manager/internal/model"
	"gorm.io/gorm"
)

const SessionCookieName = "todo_manager_session"

type DBStore struct {
	db              *gorm.DB
	sc              *securecookie.SecureCookie
	maxAge          int
	cleanupInterval time.Duration
}

func NewDBStore(db *gorm.DB, secret string, maxAge int, cleanupInterval int) *DBStore {
	hashKey := []byte(secret)
	if len(hashKey) < 32 {
		padded := make([]byte, 32)
		copy(padded, hashKey)
		hashKey = padded
	}

	s := &DBStore{
		db:              db,
		sc:              securecookie.New(hashKey, nil),
		maxAge:          maxAge,
		cleanupInterval: time.Duration(cleanupInterval) * time.Second,
	}

	go s.cleanupLoop()
	return s
}

func (s *DBStore) GetSession(r *http.Request) (string, map[string]interface{}, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", nil, nil
	}

	var sessionID string
	if err := s.sc.Decode(SessionCookieName, cookie.Value, &sessionID); err != nil {
		return "", nil, nil
	}

	var sess model.Session
	if err := s.db.Where("session_id = ? AND expires_at > ?", sessionID, time.Now()).First(&sess).Error; err != nil {
		return "", nil, nil
	}

	values, _ := decodeValues(sess.Data)
	values["user_id"] = sess.UserID

	return sessionID, values, nil
}

func (s *DBStore) CreateSession(w http.ResponseWriter, userID uint, extra map[string]interface{}) error {
	sessionID := uuid.New().String()
	values := map[string]interface{}{
		"user_id": userID,
	}
	for k, v := range extra {
		values[k] = v
	}

	data, err := encodeValues(values)
	if err != nil {
		return err
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(s.maxAge) * time.Second)

	sess := model.Session{
		SessionID: sessionID,
		UserID:    userID,
		Data:      data,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	if err := s.db.Create(&sess).Error; err != nil {
		return err
	}

	return s.setCookie(w, sessionID)
}

func (s *DBStore) DestroySession(r *http.Request, w http.ResponseWriter) {
	sessionID, _, _ := s.GetSession(r)
	if sessionID != "" {
		s.db.Where("session_id = ?", sessionID).Delete(&model.Session{})
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *DBStore) GetUserID(r *http.Request) (uint, error) {
	_, values, err := s.GetSession(r)
	if err != nil || values == nil {
		return 0, ErrNoSession
	}
	userID, ok := values["user_id"].(uint)
	if !ok {
		return 0, ErrNoSession
	}
	return userID, nil
}

func (s *DBStore) setCookie(w http.ResponseWriter, sessionID string) error {
	encoded, err := s.sc.Encode(SessionCookieName, sessionID)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   s.maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *DBStore) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		result := s.db.Where("expires_at < ?", time.Now()).Delete(&model.Session{})
		if result.RowsAffected > 0 {
			slog.Info("cleaned up expired sessions", "count", result.RowsAffected)
		}
	}
}

var ErrNoSession = &SessionError{Message: "no active session"}

type SessionError struct {
	Message string
}

func (e *SessionError) Error() string {
	return e.Message
}

func encodeValues(values map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(values); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeValues(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return make(map[string]interface{}), nil
	}
	var values map[string]interface{}
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&values); err != nil {
		return make(map[string]interface{}), nil
	}
	return values, nil
}

// verifyHMAC is a helper for signing session IDs
func verifyHMAC(key []byte, data, sig string) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// isOIDCStateCookie checks if this is an OIDC state cookie
func IsOIDCStateCookie(name string) bool {
	return strings.HasPrefix(name, "oidc_state_")
}
