package models

import (
	"event_service/internal/dto"
	"time"
)

type EventType string

const (
	EventTypeEvent EventType = "EVENT"
	EventTypeMovie EventType = "MOVIE"
)

type Event struct {
	Base

	Name            string    `gorm:"type:text;not null"`
	Description     string    `gorm:"type:text"`
	DurationMinutes int       `gorm:"not null"`
	EventType       EventType `gorm:"type:varchar(20);not null;default:EVENT"`
	Category        *string   `gorm:"type:varchar(60)"`
	Venue           *string   `gorm:"type:text"`
	City            *string   `gorm:"type:varchar(100)"`
	Address         *string   `gorm:"type:text"`
	Organizer       *string   `gorm:"type:text"`
	ImageURL        *string   `gorm:"type:text"`
	SaleOpensAt     *time.Time
	IsFlashSale     bool    `gorm:"not null;default:false"`
	Status          *string `gorm:"type:varchar(30)"`

	Director    *string    `gorm:"type:text"`
	AgeRating   *string    `gorm:"type:varchar(20)"`
	ReleaseDate *time.Time `gorm:"type:timestamptz"`
	Language    *string    `gorm:"type:varchar(50)"`
}

func (e *Event) ToDTO() *dto.EventResponse {
	return &dto.EventResponse{
		ID:              e.ID.String(),
		Name:            e.Name,
		Description:     e.Description,
		DurationMinutes: e.DurationMinutes,
		EventType:       string(e.EventType),
		Category:        e.Category,
		Venue:           e.Venue,
		City:            e.City,
		Address:         e.Address,
		Organizer:       e.Organizer,
		ImageURL:        e.ImageURL,
		SaleOpensAt:     e.SaleOpensAt,
		IsFlashSale:     e.IsFlashSale,
		Status:          e.Status,
		Director:        e.Director,
		AgeRating:       e.AgeRating,
		ReleaseDate:     e.ReleaseDate,
		Language:        e.Language,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
}
