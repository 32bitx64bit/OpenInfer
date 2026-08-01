package auth

import (
	"github.com/zalando/go-keyring"
)

const (
	keychainService = "openinfer-studio"
	keychainUser    = "huggingface-token"
)

// StoreHuggingFaceToken saves the HF token in the OS credential vault
// (Secret Service / Keychain / Credential Manager).
func StoreHuggingFaceToken(token string) error {
	return keyring.Set(keychainService, keychainUser, token)
}

// LoadHuggingFaceToken retrieves the token, returning "" when absent or when
// the vault is unavailable (the app still works for public repositories).
func LoadHuggingFaceToken() string {
	t, err := keyring.Get(keychainService, keychainUser)
	if err != nil {
		return ""
	}
	return t
}

// DeleteHuggingFaceToken removes the stored token.
func DeleteHuggingFaceToken() error {
	err := keyring.Delete(keychainService, keychainUser)
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}
