package auth

import (
	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
)

type BasicAuthProvider struct {
	users map[string]*config.BasicUser
}

func NewBasicAuthProvider(cfg *config.BasicConfig) *BasicAuthProvider {
	p := &BasicAuthProvider{
		users: make(map[string]*config.BasicUser),
	}
	for i := range cfg.Users {
		p.users[cfg.Users[i].Username] = &cfg.Users[i]
	}
	return p
}

func (p *BasicAuthProvider) Authenticate(username, password string) (*model.User, error) {
	user, ok := p.users[username]
	if !ok || user.Password != password {
		return nil, ErrInvalidCredentials
	}

	return &model.User{
		AuthProvider: "basic",
		AuthSubject:  username,
		DisplayName:  user.DisplayName,
	}, nil
}

var ErrInvalidCredentials = &AuthError{Message: "invalid username or password"}

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
