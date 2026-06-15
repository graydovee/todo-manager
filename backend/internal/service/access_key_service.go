package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/authz"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

const (
	accessKeyPrefixToken = "tdk"
	accessKeyPrefixLen   = 12
	accessKeySecretBytes = 32
	argonTime            = 1
	argonMemory          = 64 * 1024
	argonThreads         = 4
	argonKeyLen          = 32
)

type AccessKeyPermissionCatalog = authz.PermissionCatalog

type CreateAccessKeyInput struct {
	Name           string
	AuthorizedAPIs []string
	ExpiresAt      *time.Time
}

type RotateAccessKeyResult struct {
	Key      *model.AccessKey
	PlainKey string
}

type AccessKeyAuthResult struct {
	UserID         uint
	AccessKeyID    uint
	AuthorizedAPIs map[string]struct{}
}

type AccessKeyService struct {
	db            *gorm.DB
	accessKeyRepo *repository.AccessKeyRepo
	userRepo      *repository.UserRepo
}

func NewAccessKeyService(db *gorm.DB, accessKeyRepo *repository.AccessKeyRepo, userRepo *repository.UserRepo) *AccessKeyService {
	return &AccessKeyService{db: db, accessKeyRepo: accessKeyRepo, userRepo: userRepo}
}

func (s *AccessKeyService) List(userID uint) ([]*model.AccessKey, error) {
	return s.accessKeyRepo.ListByUser(nil, userID)
}

func (s *AccessKeyService) PermissionCatalog() AccessKeyPermissionCatalog {
	return authz.PermissionCatalogResponse()
}

func (s *AccessKeyService) Create(userID uint, input CreateAccessKeyInput) (*model.AccessKey, string, error) {
	authorizedAPIs, err := normalizeAuthorizedAPIs(input.AuthorizedAPIs)
	if err != nil {
		return nil, "", err
	}
	if err := validateAccessKeyInput(input.Name, authorizedAPIs, input.ExpiresAt); err != nil {
		return nil, "", err
	}

	plainKey, keyPrefix, keySalt, keyHash, err := generateAccessKeyMaterial()
	if err != nil {
		return nil, "", err
	}
	authorizedJSON, err := json.Marshal(authorizedAPIs)
	if err != nil {
		return nil, "", err
	}

	now := time.Now()
	key := &model.AccessKey{
		UserID:             userID,
		Name:               strings.TrimSpace(input.Name),
		KeyPrefix:          keyPrefix,
		KeySalt:            keySalt,
		KeyHash:            keyHash,
		AuthorizedAPIsJSON: string(authorizedJSON),
		ExpiresAt:          input.ExpiresAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.accessKeyRepo.Create(nil, key); err != nil {
		return nil, "", err
	}
	return key, plainKey, nil
}

func (s *AccessKeyService) Rotate(userID, keyID uint) (*RotateAccessKeyResult, error) {
	key, err := s.accessKeyRepo.FindByID(nil, keyID, userID)
	if err != nil {
		return nil, err
	}

	plainKey, keyPrefix, keySalt, keyHash, err := generateAccessKeyMaterial()
	if err != nil {
		return nil, err
	}

	key.KeyPrefix = keyPrefix
	key.KeySalt = keySalt
	key.KeyHash = keyHash
	key.LastUsedAt = nil
	key.UpdatedAt = time.Now()

	if err := s.accessKeyRepo.Update(nil, key); err != nil {
		return nil, err
	}

	return &RotateAccessKeyResult{Key: key, PlainKey: plainKey}, nil
}

func (s *AccessKeyService) Delete(userID, keyID uint) error {
	return s.accessKeyRepo.Delete(nil, keyID, userID)
}

func (s *AccessKeyService) Authenticate(token string) (*AccessKeyAuthResult, error) {
	keyPrefix, secret, err := parseAccessKeyToken(token)
	if err != nil {
		return nil, err
	}

	key, err := s.accessKeyRepo.FindByPrefix(nil, keyPrefix)
	if err != nil {
		return nil, err
	}
	if key.ExpiresAt != nil && !key.ExpiresAt.After(time.Now()) {
		return nil, fmt.Errorf("access key expired")
	}

	if !verifyAccessKey(secret, key.KeySalt, key.KeyHash) {
		return nil, fmt.Errorf("invalid access key")
	}

	if _, err := s.userRepo.FindByID(key.UserID); err != nil {
		return nil, err
	}

	var authorizedAPIList []string
	if err := json.Unmarshal([]byte(key.AuthorizedAPIsJSON), &authorizedAPIList); err != nil {
		return nil, fmt.Errorf("invalid stored access key permissions")
	}

	now := time.Now()
	if err := s.accessKeyRepo.TouchLastUsed(nil, key.ID, now); err != nil {
		return nil, err
	}

	authorized := make(map[string]struct{}, len(authorizedAPIList))
	for _, apiID := range authorizedAPIList {
		authorized[apiID] = struct{}{}
	}

	return &AccessKeyAuthResult{
		UserID:         key.UserID,
		AccessKeyID:    key.ID,
		AuthorizedAPIs: authorized,
	}, nil
}

func validateAccessKeyInput(name string, authorizedAPIs []string, expiresAt *time.Time) error {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return fmt.Errorf("name is required")
	}
	if len(trimmedName) > 64 {
		return fmt.Errorf("name exceeds maximum length of 64 characters")
	}
	if len(authorizedAPIs) == 0 {
		return fmt.Errorf("authorized_apis must contain at least one API")
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return fmt.Errorf("expires_at must be in the future")
	}
	return nil
}

func normalizeAuthorizedAPIs(apiIDs []string) ([]string, error) {
	if len(apiIDs) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(apiIDs))
	normalized := make([]string, 0, len(apiIDs))
	for _, apiID := range apiIDs {
		apiID = strings.TrimSpace(apiID)
		if apiID == "" {
			continue
		}
		if !authz.PermissionExists(apiID) {
			return nil, fmt.Errorf("unknown API permission: %s", apiID)
		}
		if _, ok := seen[apiID]; ok {
			continue
		}
		seen[apiID] = struct{}{}
		normalized = append(normalized, apiID)
	}
	slices.Sort(normalized)
	return normalized, nil
}

func generateAccessKeyMaterial() (plainKey, keyPrefix, keySalt, keyHash string, err error) {
	prefixRaw, err := randomBase64URL(accessKeyPrefixLen)
	if err != nil {
		return "", "", "", "", err
	}
	keyPrefix = strings.ToLower(prefixRaw[:accessKeyPrefixLen])

	secret, err := randomBase64URL(accessKeySecretBytes)
	if err != nil {
		return "", "", "", "", err
	}
	salt, err := randomBytes(16)
	if err != nil {
		return "", "", "", "", err
	}
	keySalt = base64.RawURLEncoding.EncodeToString(salt)
	keyHash = hashAccessKey(secret, salt)
	plainKey = fmt.Sprintf("%s_%s_%s", accessKeyPrefixToken, keyPrefix, secret)
	return plainKey, keyPrefix, keySalt, keyHash, nil
}

func parseAccessKeyToken(token string) (string, string, error) {
	parts := strings.SplitN(token, "_", 3)
	if len(parts) != 3 || parts[0] != accessKeyPrefixToken || len(parts[1]) != accessKeyPrefixLen || parts[2] == "" {
		return "", "", fmt.Errorf("invalid access key")
	}
	return parts[1], parts[2], nil
}

func verifyAccessKey(secret, saltEncoded, expectedHash string) bool {
	salt, err := base64.RawURLEncoding.DecodeString(saltEncoded)
	if err != nil {
		return false
	}
	return hashAccessKey(secret, salt) == expectedHash
}

func hashAccessKey(secret string, salt []byte) string {
	key := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return base64.RawURLEncoding.EncodeToString(key)
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func randomBase64URL(n int) (string, error) {
	b, err := randomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
