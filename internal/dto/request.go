package dto

import "time"

type CreateEventRequest struct {
	Name                 string     `json:"name" binding:"required"`
	Description          string     `json:"description"`
	DurationMinutes      int        `json:"duration_minutes" binding:"required,min=1"`
	EventType            string     `json:"event_type" binding:"required,oneof=EVENT MOVIE"`
	CreatorID            string     `json:"-"`
	Category             *string    `json:"category"`
	Venue                *string    `json:"venue"`
	City                 *string    `json:"city"`
	Address              *string    `json:"address"`
	Organizer            *string    `json:"organizer"`
	ImageURL             *string    `json:"image_url"`
	SaleOpensAt          *time.Time `json:"sale_opens_at"`
	IsFlashSale          bool       `json:"is_flash_sale"`
	Status               *string    `json:"status"`
	Director             *string    `json:"director"`
	AgeRating            *string    `json:"age_rating"`
	ReleaseDate          *time.Time `json:"release_date"`
	Language             *string    `json:"language"`
	TrailerURL           *string    `json:"trailer_url"`
	MaxTicketsPerBooking *int       `json:"max_tickets_per_booking"`
}

type UpdateEventRequest struct {
	Name                 string     `json:"name" binding:"required"`
	Description          string     `json:"description"`
	DurationMinutes      int        `json:"duration_minutes" binding:"required,min=1"`
	EventType            string     `json:"event_type" binding:"required,oneof=EVENT MOVIE"`
	Category             *string    `json:"category"`
	Venue                *string    `json:"venue"`
	City                 *string    `json:"city"`
	Address              *string    `json:"address"`
	Organizer            *string    `json:"organizer"`
	ImageURL             *string    `json:"image_url"`
	SaleOpensAt          *time.Time `json:"sale_opens_at"`
	IsFlashSale          bool       `json:"is_flash_sale"`
	Status               *string    `json:"status"`
	Director             *string    `json:"director"`
	AgeRating            *string    `json:"age_rating"`
	ReleaseDate          *time.Time `json:"release_date"`
	Language             *string    `json:"language"`
	TrailerURL           *string    `json:"trailer_url"`
	MaxTicketsPerBooking *int       `json:"max_tickets_per_booking"`
}

type ListEventsQuery struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
	Type     string `form:"type" binding:"omitempty,oneof=EVENT MOVIE"`
	Search   string `form:"search"`
}

type UpsertShowtimeRequest struct {
	// When set and valid for this event, the existing showtime row is updated in place (preserves seat_map_id for booking data).
	ID           *string   `json:"id,omitempty"`
	Venue        string    `json:"venue" binding:"required"`
	Address      string    `json:"address" binding:"required"`
	StartTime    time.Time `json:"start_time" binding:"required"`
	EndTime      time.Time `json:"end_time" binding:"required,gtfield=StartTime"`
	SeatMapName  string    `json:"seat_map_name"`
	QueueEnabled bool      `json:"queue_enabled"`
	QueueLimit   int       `json:"queue_limit"`
}

type CreateSeatMapRequest struct {
	Name    string                 `json:"name" binding:"required"`
	Venue   string                 `json:"venue" binding:"required"`
	Address string                 `json:"address" binding:"required"`
	Seats   []CreateSeatMapSeatDTO `json:"seats" binding:"required,min=1,dive"`
}

type CreateSeatMapSeatDTO struct {
	Row       string  `json:"row" binding:"required"`
	Number    int     `json:"number" binding:"required,min=1"`
	SeatClass string  `json:"seat_class" binding:"required,oneof=STANDARD VIP PREMIUM DELUXE"`
	Price     float64 `json:"price" binding:"required,gt=0"`
}
