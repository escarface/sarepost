package generation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/genai"
	"github.com/escarface/sarepost/internal/secure"
)

type memSettings struct {
	values map[string]string
}

func newMemSettings() *memSettings { return &memSettings{values: map[string]string{}} }

func (m *memSettings) GetSetting(_ context.Context, key string) (string, error) {
	v, ok := m.values[key]
	if !ok {
		return "", errNoRows
	}
	return v, nil
}

func (m *memSettings) SetSetting(_ context.Context, key, value string) error {
	m.values[key] = value
	return nil
}

// errNoRows mirrors sql.ErrNoRows for the in-memory store.
var errNoRows = sql.ErrNoRows

func testCipher(t *testing.T) *secure.Cipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cipher, err := secure.NewCipher(key, 1)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return cipher
}

type capturingText struct {
	gotSystem string
	gotPrompt string
}

func (c *capturingText) Name() string { return "stub" }
func (c *capturingText) GenerateText(_ context.Context, req genai.TextRequest) (genai.TextResult, error) {
	c.gotSystem = req.System
	c.gotPrompt = req.Prompt
	return genai.TextResult{Text: "generated copy", Model: "stub-model"}, nil
}

type capturingImage struct {
	gotPrompt   string
	gotRefImage []byte
	gotRefMime  string
}

func (c *capturingImage) Name() string { return "stub-image" }
func (c *capturingImage) GenerateImage(_ context.Context, req genai.ImageRequest) (genai.ImageResult, error) {
	c.gotPrompt = req.Prompt
	c.gotRefImage = req.RefImage
	c.gotRefMime = req.RefImageMime
	return genai.ImageResult{Data: []byte{0x1}, MimeType: "image/png", Model: "stub-image-model"}, nil
}

type fakeMediaReader struct {
	data map[string][]byte
	mime map[string]string
}

func (f fakeMediaReader) ReadMedia(_ context.Context, mediaID string) ([]byte, string, error) {
	d, ok := f.data[mediaID]
	if !ok {
		return nil, "", errors.New("media not found")
	}
	return d, f.mime[mediaID], nil
}

func TestGenerateText_NoProviderConfigured(t *testing.T) {
	svc := Service{Store: newMemSettings(), Cipher: testCipher(t), Driver: genai.DriverMock}
	_, err := svc.GenerateText(context.Background(), GenerateTextInput{Prompt: "hi"})
	if !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestGenerateText_AppliesBrandAndPlatform(t *testing.T) {
	store := newMemSettings()
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock}

	if _, err := svc.SaveTextProviderConfig(context.Background(), ProviderConfigUpdate{
		Provider: genai.ProviderAnthropic, Model: "claude-opus-4-8", APIKey: "sk-test",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	profile, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{
		Name: "Sare", SystemPrompt: "We sell automation.", Tone: "confident",
	})
	if err != nil {
		t.Fatalf("save brand: %v", err)
	}

	capt := &capturingText{}
	svc.TextFactory = func(_ genai.Driver, _ genai.ProviderConfig) (genai.TextProvider, error) {
		return capt, nil
	}

	out, err := svc.GenerateText(context.Background(), GenerateTextInput{
		Prompt:         "Announce our launch",
		Platform:       domain.PlatformInstagram,
		BrandProfileID: profile.ID,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out.Text != "generated copy" {
		t.Fatalf("unexpected text %q", out.Text)
	}
	if !strings.Contains(capt.gotSystem, "We sell automation.") {
		t.Errorf("system prompt missing brand context: %q", capt.gotSystem)
	}
	if !strings.Contains(capt.gotSystem, "confident") {
		t.Errorf("system prompt missing tone: %q", capt.gotSystem)
	}
	if !strings.Contains(capt.gotSystem, "Instagram") {
		t.Errorf("system prompt missing platform guidance: %q", capt.gotSystem)
	}
}

func TestGenerateText_UnknownBrandProfile(t *testing.T) {
	store := newMemSettings()
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock}
	if _, err := svc.SaveTextProviderConfig(context.Background(), ProviderConfigUpdate{
		Provider: genai.ProviderOpenAI, Model: "gpt-4o", APIKey: "sk-test",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	_, err := svc.GenerateText(context.Background(), GenerateTextInput{Prompt: "hi", BrandProfileID: "brand_missing"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestGenerateImage_UsesMockDriverWithoutKey(t *testing.T) {
	store := newMemSettings()
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock}
	// Image provider config stored without a key — mock driver ignores it.
	if _, err := svc.SaveImageProviderConfig(context.Background(), ProviderConfigUpdate{
		Provider: genai.ProviderOpenAI, Model: "gpt-image-1",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	out, err := svc.GenerateImage(context.Background(), GenerateImageInput{Prompt: "a logo"})
	if err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if len(out.Data) == 0 || out.MimeType != "image/png" {
		t.Fatalf("unexpected image result: %d bytes, %q", len(out.Data), out.MimeType)
	}
}

func TestGenerateImage_AppliesBrandReferenceImage(t *testing.T) {
	store := newMemSettings()
	reader := fakeMediaReader{
		data: map[string][]byte{"med_ref": []byte("PNGDATA")},
		mime: map[string]string{"med_ref": "image/png"},
	}
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock, MediaReader: reader}
	if _, err := svc.SaveImageProviderConfig(context.Background(), ProviderConfigUpdate{
		Provider: genai.ProviderOpenAI, Model: "gpt-image-1", APIKey: "sk-test",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	profile, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{
		Name: "Sare", ImageRefMediaID: "med_ref",
	})
	if err != nil {
		t.Fatalf("save brand: %v", err)
	}

	capt := &capturingImage{}
	svc.ImageFactory = func(_ genai.Driver, _ genai.ProviderConfig) (genai.ImageProvider, error) {
		return capt, nil
	}
	if _, err := svc.GenerateImage(context.Background(), GenerateImageInput{
		Prompt: "a banner", BrandProfileID: profile.ID,
	}); err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if string(capt.gotRefImage) != "PNGDATA" {
		t.Errorf("reference image not passed to provider: %q", capt.gotRefImage)
	}
	if capt.gotRefMime != "image/png" {
		t.Errorf("reference mime not passed: %q", capt.gotRefMime)
	}
	if !strings.Contains(capt.gotPrompt, "reference image") {
		t.Errorf("prompt missing reference hint: %q", capt.gotPrompt)
	}
}

func TestBrandProfile_KeepImageRefRetainsReference(t *testing.T) {
	store := newMemSettings()
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock}
	created, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{
		Name: "Sare", ImageRefMediaID: "med_ref",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Re-save without a media id but KeepImageRef set: reference must persist.
	updated, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{
		ID: created.ID, Name: "Sare 2", KeepImageRef: true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ImageRefMediaID != "med_ref" {
		t.Errorf("expected retained reference, got %q", updated.ImageRefMediaID)
	}
	// Re-save without KeepImageRef clears the reference.
	cleared, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{
		ID: created.ID, Name: "Sare 3",
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.ImageRefMediaID != "" {
		t.Errorf("expected cleared reference, got %q", cleared.ImageRefMediaID)
	}
}

func TestProviderConfig_KeepKeyRetainsSecret(t *testing.T) {
	store := newMemSettings()
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock}
	if _, err := svc.SaveTextProviderConfig(context.Background(), ProviderConfigUpdate{
		Provider: genai.ProviderAnthropic, Model: "claude-opus-4-8", APIKey: "sk-secret",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Re-save without a key but KeepKey set.
	if _, err := svc.SaveTextProviderConfig(context.Background(), ProviderConfigUpdate{
		Provider: genai.ProviderAnthropic, Model: "claude-opus-4-7", KeepKey: true,
	}); err != nil {
		t.Fatalf("resave: %v", err)
	}
	cfg, ok, err := svc.getProviderConfig(context.Background(), kindText)
	if err != nil || !ok {
		t.Fatalf("get config: ok=%v err=%v", ok, err)
	}
	if cfg.APIKey != "sk-secret" {
		t.Errorf("expected retained key, got %q", cfg.APIKey)
	}
	if cfg.Model != "claude-opus-4-7" {
		t.Errorf("expected updated model, got %q", cfg.Model)
	}
	view, err := svc.GetTextProviderView(context.Background())
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if !view.KeyConfigured {
		t.Errorf("expected KeyConfigured true")
	}
}

func TestBrandProfiles_CRUD(t *testing.T) {
	store := newMemSettings()
	svc := Service{Store: store, Cipher: testCipher(t), Driver: genai.DriverMock}

	created, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{Name: "B", Tone: "warm"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated id")
	}
	updated, err := svc.SaveBrandProfile(context.Background(), BrandProfileUpdate{ID: created.ID, Name: "B2"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "B2" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}
	list, err := svc.ListBrandProfiles(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("list: len=%d err=%v", len(list), err)
	}
	if err := svc.DeleteBrandProfile(context.Background(), created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = svc.ListBrandProfiles(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("list after delete: len=%d err=%v", len(list), err)
	}
}
