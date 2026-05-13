package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
	"golang.org/x/oauth2"
)

type OIDCAuthProvider struct {
	provider   *oidc.Provider
	verifier   *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	config     *config.OIDCConfig
}

type OIDCState struct {
	State        string
	Nonce        string
	CodeVerifier string
}

func NewOIDCAuthProvider(ctx context.Context, cfg *config.OIDCConfig) (*OIDCAuthProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("create OIDC provider: %w", err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes:       scopes,
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return &OIDCAuthProvider{
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Config,
		config:       cfg,
	}, nil
}

func (p *OIDCAuthProvider) GenerateState() (*OIDCState, error) {
	state, err := generateRandomString(32)
	if err != nil {
		return nil, err
	}
	nonce, err := generateRandomString(32)
	if err != nil {
		return nil, err
	}
	codeVerifier, err := generateRandomString(64)
	if err != nil {
		return nil, err
	}

	return &OIDCState{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
	}, nil
}

func (p *OIDCAuthProvider) GetAuthURL(state *OIDCState) string {
	return p.oauth2Config.AuthCodeURL(
		state.State,
		oauth2.SetAuthURLParam("nonce", state.Nonce),
		oauth2.SetAuthURLParam("code_challenge", codeVerifierToChallenge(state.CodeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (p *OIDCAuthProvider) HandleCallback(ctx context.Context, code string, state *OIDCState) (*model.User, error) {
	token, err := p.oauth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", state.CodeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("oauth2 exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	if idToken.Nonce != state.Nonce {
		return nil, fmt.Errorf("nonce mismatch")
	}

	var claims struct {
		Sub   string `json:"sub"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.Email
	}

	return &model.User{
		AuthProvider: "oidc",
		AuthSubject:  claims.Sub,
		DisplayName:  displayName,
	}, nil
}

func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeVerifierToChallenge(verifier string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(verifier))
}
