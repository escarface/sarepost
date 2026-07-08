package contentsources

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
)

func TestServiceCreateValidatesRequiredFields(t *testing.T) {
	service := Service{Store: &memoryStore{}}
	if _, err := service.Create(context.Background(), CreateInput{Title: "", Body: "body"}); !errors.Is(err, ErrTitleRequired) {
		t.Fatalf("expected title error, got %v", err)
	}
	if _, err := service.Create(context.Background(), CreateInput{Title: "title", Body: ""}); !errors.Is(err, ErrBodyRequired) {
		t.Fatalf("expected body error, got %v", err)
	}
}

func TestServiceCreateNormalizesAndPersistsSource(t *testing.T) {
	store := &memoryStore{}
	service := Service{Store: store}
	source, err := service.Create(context.Background(), CreateInput{
		Title:          "  Launch notes  ",
		Body:           "  Raw source  ",
		SourceURL:      " https://example.com ",
		CampaignID:     " cmp_123 ",
		BrandProfileID: " bp_123 ",
		Tags:           []string{" launch ", "", "launch", "founder"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if source.Title != "Launch notes" || source.Body != "Raw source" || source.SourceURL != "https://example.com" {
		t.Fatalf("source was not normalized: %#v", source)
	}
	if source.Status != domain.ContentSourceStatusNew {
		t.Fatalf("expected new status, got %q", source.Status)
	}
	if got := strings.Join(source.Tags, ","); got != "launch,founder" {
		t.Fatalf("unexpected tags %q", got)
	}
}

func TestServiceListRejectsInvalidStatus(t *testing.T) {
	service := Service{Store: &memoryStore{}}
	_, err := service.List(context.Background(), domain.ContentSourceListFilter{Status: "bad"})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected invalid status, got %v", err)
	}
}

func TestServiceArchiveMarksSourceArchived(t *testing.T) {
	store := &memoryStore{source: domain.ContentSource{ID: "src_123", Title: "Title", Body: "Body", Status: domain.ContentSourceStatusNew}}
	service := Service{Store: store}
	source, err := service.Archive(context.Background(), "src_123")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if source.Status != domain.ContentSourceStatusArchived {
		t.Fatalf("expected archived, got %q", source.Status)
	}
}

func TestServiceGenerateAnglesBuildsPromptFromSource(t *testing.T) {
	store := &memoryStore{source: domain.ContentSource{
		ID:             "src_123",
		Title:          "Transcript",
		Body:           "A customer says onboarding is too slow.",
		SourceURL:      "https://example.com/source",
		BrandProfileID: "bp_123",
		Tags:           []string{"customer", "onboarding"},
		Status:         domain.ContentSourceStatusNew,
	}}
	generator := &fakeGenerator{out: generationapp.GenerateTextOutput{Text: "1. Faster onboarding - Explain the bottleneck.", Model: "mock-model", Provider: "mock"}}
	service := Service{Store: store, Generator: generator}
	out, err := service.GenerateAngles(context.Background(), GenerateAnglesInput{ID: "src_123", Count: 3, Instructions: "Focus on founders"})
	if err != nil {
		t.Fatalf("generate angles: %v", err)
	}
	if out.Angles == "" || out.Model != "mock-model" || out.Provider != "mock" {
		t.Fatalf("unexpected output %#v", out)
	}
	if generator.input.BrandProfileID != "bp_123" {
		t.Fatalf("expected brand profile passed through, got %q", generator.input.BrandProfileID)
	}
	for _, want := range []string{"Generate 3", "Transcript", "https://example.com/source", "customer, onboarding", "Focus on founders", "onboarding is too slow"} {
		if !strings.Contains(generator.input.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, generator.input.Prompt)
		}
	}
}

type memoryStore struct {
	source domain.ContentSource
}

func (m *memoryStore) CreateContentSource(ctx context.Context, source domain.ContentSource) (domain.ContentSource, error) {
	source.ID = "src_created"
	m.source = source
	return source, nil
}

func (m *memoryStore) ListContentSources(ctx context.Context, filter domain.ContentSourceListFilter) ([]domain.ContentSource, error) {
	if m.source.ID == "" {
		return nil, nil
	}
	return []domain.ContentSource{m.source}, nil
}

func (m *memoryStore) GetContentSource(ctx context.Context, id string) (domain.ContentSource, error) {
	if m.source.ID == "" || id != m.source.ID {
		return domain.ContentSource{}, sql.ErrNoRows
	}
	return m.source, nil
}

func (m *memoryStore) UpdateContentSource(ctx context.Context, source domain.ContentSource) (domain.ContentSource, error) {
	m.source = source
	return source, nil
}

type fakeGenerator struct {
	input generationapp.GenerateTextInput
	out   generationapp.GenerateTextOutput
}

func (f *fakeGenerator) GenerateText(ctx context.Context, in generationapp.GenerateTextInput) (generationapp.GenerateTextOutput, error) {
	f.input = in
	return f.out, nil
}
