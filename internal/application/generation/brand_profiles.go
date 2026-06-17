package generation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// BrandProfile captures reusable brand voice/context applied to generation.
type BrandProfile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	Tone         string `json:"tone,omitempty"`
	// ImageRefMediaID is the id of an uploaded media item used as a visual style
	// reference when generating images for this brand.
	ImageRefMediaID string `json:"image_ref_media_id,omitempty"`
}

// BrandProfileUpdate is the create/update input. An empty ID creates a new one.
type BrandProfileUpdate struct {
	ID           string
	Name         string
	SystemPrompt string
	Tone         string
	// ImageRefMediaID sets the reference image. When empty and KeepImageRef is
	// true, the stored reference is retained (used on edit when no new file is
	// uploaded); when empty and KeepImageRef is false, the reference is cleared.
	ImageRefMediaID string
	KeepImageRef    bool
}

// ListBrandProfiles returns the stored brand profiles sorted by name.
func (s Service) ListBrandProfiles(ctx context.Context) ([]BrandProfile, error) {
	profiles, err := s.loadBrandProfiles(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(profiles, func(i, j int) bool {
		return strings.ToLower(profiles[i].Name) < strings.ToLower(profiles[j].Name)
	})
	return profiles, nil
}

// GetBrandProfile returns one profile by ID.
func (s Service) GetBrandProfile(ctx context.Context, id string) (BrandProfile, bool, error) {
	profiles, err := s.loadBrandProfiles(ctx)
	if err != nil {
		return BrandProfile{}, false, err
	}
	for _, p := range profiles {
		if p.ID == id {
			return p, true, nil
		}
	}
	return BrandProfile{}, false, nil
}

// ResolveBrandProfileID returns an explicit profile ID or resolves a profile by name.
func (s Service) ResolveBrandProfileID(ctx context.Context, id string, name string) (string, error) {
	id = strings.TrimSpace(id)
	if id != "" {
		return id, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	profiles, err := s.loadBrandProfiles(ctx)
	if err != nil {
		return "", err
	}
	var matches []BrandProfile
	for _, profile := range profiles {
		if strings.EqualFold(strings.TrimSpace(profile.Name), name) {
			matches = append(matches, profile)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("brand profile %q not found", name)
	case 1:
		return matches[0].ID, nil
	default:
		return "", fmt.Errorf("brand profile %q is ambiguous", name)
	}
}

// SaveBrandProfile creates or updates a profile and returns it.
func (s Service) SaveBrandProfile(ctx context.Context, update BrandProfileUpdate) (BrandProfile, error) {
	if s.Store == nil {
		return BrandProfile{}, errors.New("settings store is required")
	}
	name := strings.TrimSpace(update.Name)
	if name == "" {
		return BrandProfile{}, errors.New("brand profile name is required")
	}
	profiles, err := s.loadBrandProfiles(ctx)
	if err != nil {
		return BrandProfile{}, err
	}
	profile := BrandProfile{
		ID:              strings.TrimSpace(update.ID),
		Name:            name,
		SystemPrompt:    strings.TrimSpace(update.SystemPrompt),
		Tone:            strings.TrimSpace(update.Tone),
		ImageRefMediaID: strings.TrimSpace(update.ImageRefMediaID),
	}
	if profile.ImageRefMediaID == "" && update.KeepImageRef && profile.ID != "" {
		for i := range profiles {
			if profiles[i].ID == profile.ID {
				profile.ImageRefMediaID = profiles[i].ImageRefMediaID
				break
			}
		}
	}
	if profile.ID == "" {
		id, err := newBrandProfileID()
		if err != nil {
			return BrandProfile{}, err
		}
		profile.ID = id
		profiles = append(profiles, profile)
	} else {
		found := false
		for i := range profiles {
			if profiles[i].ID == profile.ID {
				profiles[i] = profile
				found = true
				break
			}
		}
		if !found {
			return BrandProfile{}, fmt.Errorf("brand profile %s not found", profile.ID)
		}
	}
	if err := s.storeBrandProfiles(ctx, profiles); err != nil {
		return BrandProfile{}, err
	}
	return profile, nil
}

// DeleteBrandProfile removes a profile by ID.
func (s Service) DeleteBrandProfile(ctx context.Context, id string) error {
	if s.Store == nil {
		return errors.New("settings store is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("brand profile id is required")
	}
	profiles, err := s.loadBrandProfiles(ctx)
	if err != nil {
		return err
	}
	out := profiles[:0]
	removed := false
	for _, p := range profiles {
		if p.ID == id {
			removed = true
			continue
		}
		out = append(out, p)
	}
	if !removed {
		return fmt.Errorf("brand profile %s not found", id)
	}
	return s.storeBrandProfiles(ctx, out)
}

func (s Service) loadBrandProfiles(ctx context.Context) ([]BrandProfile, error) {
	if s.Store == nil {
		return nil, errors.New("settings store is required")
	}
	raw, err := s.Store.GetSetting(ctx, SettingBrandProfiles)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var profiles []BrandProfile
	if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
		return nil, fmt.Errorf("decode brand profiles: %w", err)
	}
	return profiles, nil
}

func (s Service) storeBrandProfiles(ctx context.Context, profiles []BrandProfile) error {
	raw, err := json.Marshal(profiles)
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, SettingBrandProfiles, string(raw))
}

func newBrandProfileID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "brand_" + hex.EncodeToString(buf), nil
}
