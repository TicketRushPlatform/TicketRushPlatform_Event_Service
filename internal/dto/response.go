package dto

import "time"

type EventResponse struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	DurationMinutes int        `json:"duration_minutes"`
	EventType       string     `json:"event_type"`
	Category        *string    `json:"category,omitempty"`
	Venue           *string    `json:"venue,omitempty"`
	City            *string    `json:"city,omitempty"`
	Address         *string    `json:"address,omitempty"`
	Organizer       *string    `json:"organizer,omitempty"`
	ImageURL        *string    `json:"image_url,omitempty"`
	SaleOpensAt     *time.Time `json:"sale_opens_at,omitempty"`
	IsFlashSale     bool       `json:"is_flash_sale"`
	Status          *string    `json:"status,omitempty"`
	Director        *string    `json:"director,omitempty"`
	AgeRating       *string    `json:"age_rating,omitempty"`
	ReleaseDate     *time.Time `json:"release_date,omitempty"`
	Language        *string    `json:"language,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ShowtimeResponse struct {
	ID          string    `json:"id"`
	EventID     string    `json:"event_id"`
	Venue       string    `json:"venue"`
	Address     string    `json:"address"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	SeatMapName string    `json:"seat_map_name"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalItems int64       `json:"total_items"`
	TotalPages int         `json:"total_pages"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
