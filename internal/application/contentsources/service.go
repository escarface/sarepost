package contentsources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/genai"
)

var (
	ErrTitleRequired    = errors.New("content source title is required")
	ErrBodyRequired     = errors.New("content source body is required")
	ErrIDRequired       = errors.New("content source id is required")
	ErrInvalidStatus    = errors.New("content source status is invalid")
	ErrSourceNotFound   = errors.New("content source not found")
	ErrGeneratorMissing = errors.New("content source generator is not configured")
)

type Store interface {
	CreateContentSource(ctx context.Context, source domain.ContentSource) (domain.ContentSource, error)
	ListContentSources(ctx context.Context, filter domain.ContentSourceListFilter) ([]domain.ContentSource, error)
	GetContentSource(ctx context.Context, id string) (domain.ContentSource, error)
	UpdateContentSource(ctx context.Context, source domain.ContentSource) (domain.ContentSource, error)
}

type TextGenerator interface {
	GenerateText(ctx context.Context, in generationapp.GenerateTextInput) (generationapp.GenerateTextOutput, error)
}

type Service struct {
	Store     Store
	Generator TextGenerator
}

type CreateInput struct {
	Title          string
	Body           string
	SourceURL      string
	CampaignID     string
	BrandProfileID string
	Tags           []string
}

type UpdateInput struct {
	ID             string
	Title          string
	Body           string
	SourceURL      string
	CampaignID     string
	BrandProfileID string
	Tags           []string
	Status         domain.ContentSourceStatus
}

type GenerateAnglesInput struct {
	ID           string
	Count        int
	Instructions string
}

type GenerateAnglesOutput struct {
	SourceID      string             `json:"source_id"`
	Angles        string             `json:"angles"`
	Model         string             `json:"model"`
	Provider      string             `json:"provider"`
	UsedWebSearch bool               `json:"used_web_search"`
	Sources       []genai.TextSource `json:"sources,omitempty"`
}

func (s Service) Create(ctx context.Context, in CreateInput) (domain.ContentSource, error) {
	source := domain.ContentSource{
		Title:          strings.TrimSpace(in.Title),
		Body:           strings.TrimSpace(in.Body),
		SourceURL:      strings.TrimSpace(in.SourceURL),
		CampaignID:     strings.TrimSpace(in.CampaignID),
		BrandProfileID: strings.TrimSpace(in.BrandProfileID),
		Tags:           normalizeTags(in.Tags),
		Status:         domain.ContentSourceStatusNew,
	}
	if err := validateSource(source); err != nil {
		return domain.ContentSource{}, err
	}
	return s.Store.CreateContentSource(ctx, source)
}

func (s Service) List(ctx context.Context, filter domain.ContentSourceListFilter) ([]domain.ContentSource, error) {
	filter.Tag = strings.TrimSpace(filter.Tag)
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, ErrInvalidStatus
	}
	return s.Store.ListContentSources(ctx, filter)
}

func (s Service) Get(ctx context.Context, id string) (domain.ContentSource, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ContentSource{}, ErrIDRequired
	}
	source, err := s.Store.GetContentSource(ctx, id)
	if err != nil {
		return domain.ContentSource{}, mapNotFound(err)
	}
	return source, nil
}

func (s Service) Update(ctx context.Context, in UpdateInput) (domain.ContentSource, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return domain.ContentSource{}, ErrIDRequired
	}
	current, err := s.Store.GetContentSource(ctx, id)
	if err != nil {
		return domain.ContentSource{}, mapNotFound(err)
	}
	if strings.TrimSpace(in.Title) != "" {
		current.Title = strings.TrimSpace(in.Title)
	}
	if strings.TrimSpace(in.Body) != "" {
		current.Body = strings.TrimSpace(in.Body)
	}
	current.SourceURL = strings.TrimSpace(in.SourceURL)
	current.CampaignID = strings.TrimSpace(in.CampaignID)
	current.BrandProfileID = strings.TrimSpace(in.BrandProfileID)
	current.Tags = normalizeTags(in.Tags)
	if in.Status != "" {
		if !validStatus(in.Status) {
			return domain.ContentSource{}, ErrInvalidStatus
		}
		current.Status = in.Status
	}
	if err := validateSource(current); err != nil {
		return domain.ContentSource{}, err
	}
	return s.Store.UpdateContentSource(ctx, current)
}

func (s Service) Archive(ctx context.Context, id string) (domain.ContentSource, error) {
	source, err := s.Get(ctx, id)
	if err != nil {
		return domain.ContentSource{}, err
	}
	source.Status = domain.ContentSourceStatusArchived
	return s.Store.UpdateContentSource(ctx, source)
}

func (s Service) GenerateAngles(ctx context.Context, in GenerateAnglesInput) (GenerateAnglesOutput, error) {
	if s.Generator == nil {
		return GenerateAnglesOutput{}, ErrGeneratorMissing
	}
	source, err := s.Get(ctx, in.ID)
	if err != nil {
		return GenerateAnglesOutput{}, err
	}
	count := in.Count
	if count <= 0 {
		count = 5
	}
	if count > 10 {
		count = 10
	}
	generated, err := s.Generator.GenerateText(ctx, generationapp.GenerateTextInput{
		Prompt:         buildAnglesPrompt(source, count, in.Instructions),
		BrandProfileID: source.BrandProfileID,
		MaxTokens:      700,
	})
	if err != nil {
		return GenerateAnglesOutput{}, err
	}
	return GenerateAnglesOutput{
		SourceID:      source.ID,
		Angles:        strings.TrimSpace(generated.Text),
		Model:         generated.Model,
		Provider:      generated.Provider,
		UsedWebSearch: generated.UsedWebSearch,
		Sources:       append([]genai.TextSource(nil), generated.Sources...),
	}, nil
}

func validateSource(source domain.ContentSource) error {
	if strings.TrimSpace(source.Title) == "" {
		return ErrTitleRequired
	}
	if strings.TrimSpace(source.Body) == "" {
		return ErrBodyRequired
	}
	if source.Status == "" {
		return ErrInvalidStatus
	}
	if !validStatus(source.Status) {
		return ErrInvalidStatus
	}
	return nil
}

func validStatus(status domain.ContentSourceStatus) bool {
	switch status {
	case domain.ContentSourceStatusNew, domain.ContentSourceStatusProcessed, domain.ContentSourceStatusArchived:
		return true
	default:
		return false
	}
}

func buildAnglesPrompt(source domain.ContentSource, count int, instructions string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Generate %d concise editorial angles from this content source.\n", count)
	b.WriteString("Do not write final social posts. Return a numbered list where each item has a short angle title and one-sentence rationale.\n\n")
	fmt.Fprintf(&b, "Title: %s\n", source.Title)
	if source.SourceURL != "" {
		fmt.Fprintf(&b, "Reference URL: %s\n", source.SourceURL)
	}
	if len(source.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(source.Tags, ", "))
	}
	if trimmed := strings.TrimSpace(instructions); trimmed != "" {
		fmt.Fprintf(&b, "Additional instructions: %s\n", trimmed)
	}
	b.WriteString("\nSource material:\n")
	b.WriteString(source.Body)
	return b.String()
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSourceNotFound
	}
	return err
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}
