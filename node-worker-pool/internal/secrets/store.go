package secrets

import (
	"errors"
	"os"
)

// SecretStore provides access to secrets
type SecretStore interface {
	Get(key string) (string, error)
}

// EnvSecretStore fetches secrets from environment variables
type EnvSecretStore struct{}

// NewEnvSecretStore creates a new environment-based secret store
func NewEnvSecretStore() *EnvSecretStore {
	return &EnvSecretStore{}
}

// Get retrieves a secret from environment variables
func (s *EnvSecretStore) Get(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", errors.New("secret not found: " + key)
	}
	return value, nil
}
