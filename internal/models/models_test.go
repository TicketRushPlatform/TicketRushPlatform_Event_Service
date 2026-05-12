package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestBaseBeforeCreate(t *testing.T) {
	tests := []struct {
		name string
		base Base
	}{
		{name: "assign new uuid", base: Base{}},
		{name: "keep existing uuid", base: Base{ID: uuid.New()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldID := tt.base.ID
			if err := tt.base.BeforeCreate(&gorm.DB{}); err != nil {
				t.Fatalf("BeforeCreate() error = %v", err)
			}

			if oldID == uuid.Nil && tt.base.ID == uuid.Nil {
				t.Fatalf("expected ID to be generated")
			}
			if oldID != uuid.Nil && tt.base.ID != oldID {
				t.Fatalf("expected existing ID unchanged")
			}
		})
	}
}

func TestEventToDTO(t *testing.T) {
	now := time.Now().UTC()
	director := "Nolan"
	rating := "PG-13"
	language := "EN"
	trailerURL := "https://www.youtube.com/embed/example"
	release := now.Add(-48 * time.Hour)

	e := Event{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:            "Interstellar",
		Description:     "Sci-fi",
		DurationMinutes: 169,
		EventType:       EventTypeMovie,
		Director:        &director,
		AgeRating:       &rating,
		ReleaseDate:     &release,
		Language:        &language,
		TrailerURL:      &trailerURL,
	}

	dto := e.ToDTO()
	if dto.ID == "" || dto.Name != e.Name || dto.EventType != string(EventTypeMovie) {
		t.Fatalf("unexpected dto mapping: %+v", dto)
	}
	if dto.TrailerURL == nil || *dto.TrailerURL != trailerURL {
		t.Fatalf("expected trailer URL to be mapped, got %+v", dto.TrailerURL)
	}
}

func TestEventReviewToDTO(t *testing.T) {
	now := time.Now().UTC()
	review := EventReview{
		Base: Base{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		EventID:    uuid.New(),
		UserID:     uuid.New(),
		AuthorName: "  Minh Anh  ",
		Rating:     5,
		Comment:    "Great event.",
	}

	dto := review.ToDTO()
	if dto.ID != review.ID.String() || dto.EventID != review.EventID.String() || dto.UserID != review.UserID.String() {
		t.Fatalf("unexpected dto ids: %+v", dto)
	}
	if dto.AuthorName != "Minh Anh" || dto.Rating != 5 || dto.Comment != review.Comment {
		t.Fatalf("unexpected review dto mapping: %+v", dto)
	}
}
