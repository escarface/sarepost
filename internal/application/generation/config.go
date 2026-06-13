package generation

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/escarface/sarepost/internal/genai"
)

// Settings keys for the generation feature.
const (
	SettingGenerationText  = "generation.text"
	SettingGenerationImage = "generation.image"
	SettingBrandProfiles   = "generation.brand_profiles"
)

// SettingsStore is the persistence dependency (satisfied by *db.Store).
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// ProviderConfig is the resolved provider configuration including the secret.
type ProviderConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`
}

// ProviderConfigView hides the API key; it only reports whether one is set.
type ProviderConfigView struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	KeyConfigured bool   `json:"key_configured"`
	Configured    bool   `json:"configured"`
}

// ProviderConfigUpdate is the input from the settings form. KeepKey preserves
// the stored key when the form leaves the key field blank.
type ProviderConfigUpdate struct {
	Provider string
	Model    string
	APIKey   string
	KeepKey  bool
}

type storedProviderConfig struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	KeyCiphertext string `json:"key_ciphertext,omitempty"`
	KeyNonce      string `json:"key_nonce,omitempty"`
	KeyKeyVersion int    `json:"key_key_version,omitempty"`
}

func (s Service) settingKeyFor(kind providerKind) string {
	if kind == kindImage {
		return SettingGenerationImage
	}
	return SettingGenerationText
}

type providerKind int

const (
	kindText providerKind = iota
	kindImage
)

// GetProviderConfig loads and decrypts the stored provider config.
func (s Service) getProviderConfig(ctx context.Context, kind providerKind) (ProviderConfig, bool, error) {
	stored, ok, err := s.loadStoredProviderConfig(ctx, kind)
	if err != nil || !ok {
		return ProviderConfig{}, ok, err
	}
	key, err := s.decryptKey(stored)
	if err != nil {
		return ProviderConfig{}, true, err
	}
	return ProviderConfig{Provider: stored.Provider, Model: stored.Model, APIKey: key}, true, nil
}

// GetTextProviderView returns the text provider config without the secret.
func (s Service) GetTextProviderView(ctx context.Context) (ProviderConfigView, error) {
	return s.providerView(ctx, kindText)
}

// GetImageProviderView returns the image provider config without the secret.
func (s Service) GetImageProviderView(ctx context.Context) (ProviderConfigView, error) {
	return s.providerView(ctx, kindImage)
}

func (s Service) providerView(ctx context.Context, kind providerKind) (ProviderConfigView, error) {
	cfg, ok, err := s.getProviderConfig(ctx, kind)
	if err != nil {
		return ProviderConfigView{}, err
	}
	if !ok {
		return ProviderConfigView{}, nil
	}
	return ProviderConfigView{
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		KeyConfigured: cfg.APIKey != "",
		Configured:    true,
	}, nil
}

// SaveTextProviderConfig persists the text provider config.
func (s Service) SaveTextProviderConfig(ctx context.Context, update ProviderConfigUpdate) (ProviderConfigView, error) {
	return s.saveProviderConfig(ctx, kindText, update, genai.SupportedTextProviders())
}

// SaveImageProviderConfig persists the image provider config.
func (s Service) SaveImageProviderConfig(ctx context.Context, update ProviderConfigUpdate) (ProviderConfigView, error) {
	return s.saveProviderConfig(ctx, kindImage, update, genai.SupportedImageProviders())
}

func (s Service) saveProviderConfig(ctx context.Context, kind providerKind, update ProviderConfigUpdate, allowed []string) (ProviderConfigView, error) {
	if s.Store == nil {
		return ProviderConfigView{}, errors.New("settings store is required")
	}
	provider := strings.TrimSpace(strings.ToLower(update.Provider))
	if !contains(allowed, provider) {
		return ProviderConfigView{}, fmt.Errorf("unsupported provider %q", update.Provider)
	}
	cfg := ProviderConfig{
		Provider: provider,
		Model:    strings.TrimSpace(update.Model),
		APIKey:   strings.TrimSpace(update.APIKey),
	}
	if update.KeepKey && cfg.APIKey == "" {
		if existing, ok, err := s.getProviderConfig(ctx, kind); err != nil {
			return ProviderConfigView{}, err
		} else if ok {
			cfg.APIKey = existing.APIKey
		}
	}
	stored, err := s.storedFromProviderConfig(cfg)
	if err != nil {
		return ProviderConfigView{}, err
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return ProviderConfigView{}, err
	}
	if err := s.Store.SetSetting(ctx, s.settingKeyFor(kind), string(raw)); err != nil {
		return ProviderConfigView{}, err
	}
	return s.providerView(ctx, kind)
}

func (s Service) loadStoredProviderConfig(ctx context.Context, kind providerKind) (storedProviderConfig, bool, error) {
	if s.Store == nil {
		return storedProviderConfig{}, false, errors.New("settings store is required")
	}
	raw, err := s.Store.GetSetting(ctx, s.settingKeyFor(kind))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedProviderConfig{}, false, nil
		}
		return storedProviderConfig{}, false, err
	}
	var stored storedProviderConfig
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return storedProviderConfig{}, true, fmt.Errorf("decode generation config: %w", err)
	}
	return stored, true, nil
}

func (s Service) storedFromProviderConfig(cfg ProviderConfig) (storedProviderConfig, error) {
	stored := storedProviderConfig{Provider: cfg.Provider, Model: cfg.Model}
	if cfg.APIKey == "" {
		return stored, nil
	}
	if s.Cipher == nil {
		return storedProviderConfig{}, errors.New("cipher is required to store api key")
	}
	ciphertext, nonce, err := s.Cipher.EncryptJSON(cfg.APIKey)
	if err != nil {
		return storedProviderConfig{}, err
	}
	stored.KeyCiphertext = base64.StdEncoding.EncodeToString(ciphertext)
	stored.KeyNonce = base64.StdEncoding.EncodeToString(nonce)
	stored.KeyKeyVersion = s.Cipher.KeyVersion()
	return stored, nil
}

func (s Service) decryptKey(stored storedProviderConfig) (string, error) {
	if stored.KeyCiphertext == "" && stored.KeyNonce == "" {
		return "", nil
	}
	if s.Cipher == nil {
		return "", errors.New("cipher is required to read api key")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(stored.KeyCiphertext)
	if err != nil {
		return "", fmt.Errorf("decode api key ciphertext: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(stored.KeyNonce)
	if err != nil {
		return "", fmt.Errorf("decode api key nonce: %w", err)
	}
	var key string
	if err := s.Cipher.DecryptJSON(ciphertext, nonce, &key); err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}
	return key, nil
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
