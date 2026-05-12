package models

import (
	"event_service/internal/dto"
	"strings"

	"github.com/google/uuid"
)

type EventReview struct {
	Base
	EventID    uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	AuthorName string    `gorm:"type:varchar(160);not null"`
	Rating     int       `gorm:"not null"`
	Comment    string    `gorm:"type:text;not null"`
}

func (r *EventReview) ToDTO() *dto.EventReviewResponse {
	return &dto.EventReviewResponse{
		ID:         r.ID.String(),
		EventID:    r.EventID.String(),
		UserID:     r.UserID.String(),
		AuthorName: strings.TrimSpace(r.AuthorName),
		Rating:     r.Rating,
		Comment:    r.Comment,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}
