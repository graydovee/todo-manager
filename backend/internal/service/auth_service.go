package service

import (
	"context"
	"net/http"

	"github.com/graydovee/todolist/internal/auth"
	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"github.com/graydovee/todolist/internal/session"
)

type AuthService struct {
	cfg         *config.Config
	basicAuth   *auth.BasicAuthProvider
	oidcAuth    *auth.OIDCAuthProvider
	userRepo    *repository.UserRepo
	sessionStore *session.DBStore
}

func NewAuthService(
	cfg *config.Config,
	basicAuth *auth.BasicAuthProvider,
	oidcAuth *auth.OIDCAuthProvider,
	userRepo *repository.UserRepo,
	sessionStore *session.DBStore,
) *AuthService {
	return &AuthService{
		cfg:          cfg,
		basicAuth:    basicAuth,
		oidcAuth:     oidcAuth,
		userRepo:     userRepo,
		sessionStore: sessionStore,
	}
}

func (s *AuthService) LoginBasic(w http.ResponseWriter, r *http.Request, username, password string) (*model.User, error) {
	userInfo, err := s.basicAuth.Authenticate(username, password)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.UpsertByAuthProvider(userInfo)
	if err != nil {
		return nil, err
	}

	if err := s.sessionStore.CreateSession(w, user.ID, nil); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) InitOIDCLogin(w http.ResponseWriter, r *http.Request) (string, error) {
	state, err := s.oidcAuth.GenerateState()
	if err != nil {
		return "", err
	}

	// Store state in a short-lived cookie for validation on callback
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state_" + state.State,
		Value:    state.Nonce + ":" + state.CodeVerifier,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return s.oidcAuth.GetAuthURL(state), nil
}

func (s *AuthService) HandleOIDCCallback(ctx context.Context, w http.ResponseWriter, r *http.Request, code, state string) (*model.User, error) {
	// Retrieve state cookie
	cookie, err := r.Cookie("oidc_state_" + state)
	if err != nil {
		return nil, auth.ErrInvalidCredentials
	}

	parts := splitStateCookie(cookie.Value)
	if len(parts) != 2 {
		return nil, auth.ErrInvalidCredentials
	}

	oidcState := &auth.OIDCState{
		State:        state,
		Nonce:        parts[0],
		CodeVerifier: parts[1],
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state_" + state,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	userInfo, err := s.oidcAuth.HandleCallback(ctx, code, oidcState)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.UpsertByAuthProvider(userInfo)
	if err != nil {
		return nil, err
	}

	if err := s.sessionStore.CreateSession(w, user.ID, nil); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	s.sessionStore.DestroySession(r, w)
}

func (s *AuthService) GetCurrentUser(w http.ResponseWriter, r *http.Request) (*model.User, error) {
	userID, err := s.sessionStore.GetUserID(r)
	if err != nil {
		return nil, err
	}
	return s.userRepo.FindByID(userID)
}

func (s *AuthService) GetAuthMode() string {
	return s.cfg.Auth.Mode
}

func splitStateCookie(val string) []string {
	idx := 0
	for i, c := range val {
		if c == ':' {
			return []string{val[:i], val[i+1:]}
		}
		_ = idx
	}
	return nil
}
