package repository

import (
	"event_service/internal/apperror"
	"event_service/internal/dto"
	"event_service/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type EventRepository interface {
	Create(req dto.CreateEventRequest) (*models.Event, error)
	GetByID(eventID uuid.UUID) (*models.Event, error)
	List(query dto.ListEventsQuery) ([]models.Event, int64, error)
	GetShowtimeByID(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error)
	ListShowtimesByEventID(eventID uuid.UUID) ([]dto.ShowtimeResponse, error)
	ReplaceShowtimesByEventID(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error)
	Update(eventID uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error)
	Delete(eventID uuid.UUID) error
}

type eventRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewEventRepository(db *gorm.DB, logger *zap.Logger) EventRepository {
	return &eventRepository{
		db:     db,
		logger: logger,
	}
}

func (r *eventRepository) Create(req dto.CreateEventRequest) (*models.Event, error) {
	event := &models.Event{
		Name:            req.Name,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		EventType:       models.EventType(req.EventType),
		Category:        req.Category,
		Venue:           req.Venue,
		City:            req.City,
		Address:         req.Address,
		Organizer:       req.Organizer,
		ImageURL:        req.ImageURL,
		SaleOpensAt:     req.SaleOpensAt,
		IsFlashSale:     req.IsFlashSale,
		Status:          req.Status,
		Director:        req.Director,
		AgeRating:       req.AgeRating,
		ReleaseDate:     req.ReleaseDate,
		Language:        req.Language,
	}

	if err := r.db.Create(event).Error; err != nil {
		return nil, apperror.NewInternal("failed to create event", err)
	}

	return event, nil
}

func (r *eventRepository) GetByID(eventID uuid.UUID) (*models.Event, error) {
	var event models.Event
	if err := r.db.First(&event, "id = ? AND deleted_at IS NULL", eventID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.NewNotFound("event not found")
		}

		return nil, apperror.NewInternal("failed to get event", err)
	}

	return &event, nil
}

func (r *eventRepository) List(query dto.ListEventsQuery) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64

	q := r.db.Model(&models.Event{}).Where("deleted_at IS NULL")
	if query.Type != "" {
		q = q.Where("event_type = ?", query.Type)
	}

	if query.Search != "" {
		q = q.Where("name ILIKE ?", "%"+query.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, apperror.NewInternal("failed to count events", err)
	}

	offset := (query.Page - 1) * query.PageSize
	if err := q.
		Order("created_at DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&events).Error; err != nil {
		return nil, 0, apperror.NewInternal("failed to list events", err)
	}

	return events, total, nil
}

func (r *eventRepository) Update(eventID uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error) {
	event, err := r.GetByID(eventID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":             req.Name,
		"description":      req.Description,
		"duration_minutes": req.DurationMinutes,
		"event_type":       req.EventType,
		"category":         req.Category,
		"venue":            req.Venue,
		"city":             req.City,
		"address":          req.Address,
		"organizer":        req.Organizer,
		"image_url":        req.ImageURL,
		"sale_opens_at":    req.SaleOpensAt,
		"is_flash_sale":    req.IsFlashSale,
		"status":           req.Status,
		"director":         req.Director,
		"age_rating":       req.AgeRating,
		"release_date":     req.ReleaseDate,
		"language":         req.Language,
	}

	if err := r.db.Model(event).Updates(updates).Error; err != nil {
		return nil, apperror.NewInternal("failed to update event", err)
	}

	if err := r.db.First(event, "id = ? AND deleted_at IS NULL", eventID).Error; err != nil {
		return nil, apperror.NewInternal("failed to get updated event", err)
	}

	return event, nil
}

func (r *eventRepository) GetShowtimeByID(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
	var showtime dto.ShowtimeResponse
	err := r.db.Raw(`
		SELECT
			st.id::text AS id,
			st.event_id::text AS event_id,
			v.name AS venue,
			v.address AS address,
			st.start_time,
			st.end_time,
			sm.name AS seat_map_name
		FROM show_times st
		INNER JOIN seat_maps sm ON sm.id = st.seat_map_id
		INNER JOIN venues v ON v.id = sm.venue_id
		WHERE st.id = ? AND st.deleted_at IS NULL
		LIMIT 1
	`, showtimeID).Scan(&showtime).Error
	if err != nil {
		return nil, apperror.NewInternal("failed to get showtime", err)
	}
	if showtime.ID == "" {
		return nil, apperror.NewNotFound("showtime not found")
	}
	return &showtime, nil
}

func (r *eventRepository) ListShowtimesByEventID(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
	showtimes := make([]dto.ShowtimeResponse, 0)
	err := r.db.Raw(`
		SELECT
			st.id::text AS id,
			st.event_id::text AS event_id,
			v.name AS venue,
			v.address AS address,
			st.start_time,
			st.end_time,
			sm.name AS seat_map_name
		FROM show_times st
		INNER JOIN seat_maps sm ON sm.id = st.seat_map_id
		INNER JOIN venues v ON v.id = sm.venue_id
		WHERE st.event_id = ? AND st.deleted_at IS NULL
		ORDER BY st.start_time ASC
	`, eventID).Scan(&showtimes).Error
	if err != nil {
		return nil, apperror.NewInternal("failed to list showtimes", err)
	}
	return showtimes, nil
}

func (r *eventRepository) Delete(eventID uuid.UUID) error {
	result := r.db.Where("id = ? AND deleted_at IS NULL", eventID).Delete(&models.Event{})
	if result.Error != nil {
		return apperror.NewInternal("failed to delete event", result.Error)
	}

	if result.RowsAffected == 0 {
		return apperror.NewNotFound("event not found")
	}

	return nil
}

func (r *eventRepository) ReplaceShowtimesByEventID(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
	if _, err := r.GetByID(eventID); err != nil {
		return nil, err
	}
	tx := r.db.Begin()
	if tx.Error != nil {
		return nil, apperror.NewInternal("failed to start transaction", tx.Error)
	}
	defer func() {
		if recover() != nil {
			tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	if err := tx.Exec(`UPDATE show_times SET deleted_at = ?, updated_at = ? WHERE event_id = ? AND deleted_at IS NULL`, now, now, eventID).Error; err != nil {
		tx.Rollback()
		return nil, apperror.NewInternal("failed to clear old showtimes", err)
	}

	for index, item := range showtimes {
		venueID := uuid.New()
		seatMapID := uuid.New()
		showtimeID := uuid.New()
		seatMapName := item.SeatMapName
		if seatMapName == "" {
			seatMapName = fmt.Sprintf("Auto map %d", index+1)
		}

		if err := tx.Exec(
			`INSERT INTO venues (id, name, address, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			venueID, item.Venue, item.Address, now, now,
		).Error; err != nil {
			tx.Rollback()
			return nil, apperror.NewInternal("failed to create venue for showtime", err)
		}

		if err := tx.Exec(
			`INSERT INTO seat_maps (id, name, venue_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			seatMapID, seatMapName, venueID, now, now,
		).Error; err != nil {
			tx.Rollback()
			return nil, apperror.NewInternal("failed to create seat map for showtime", err)
		}

		if err := tx.Exec(
			`INSERT INTO show_times (id, event_id, seat_map_id, start_time, end_time, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			showtimeID, eventID, seatMapID, item.StartTime, item.EndTime, now, now,
		).Error; err != nil {
			tx.Rollback()
			return nil, apperror.NewInternal("failed to create showtime", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, apperror.NewInternal("failed to commit showtime updates", err)
	}
	return r.ListShowtimesByEventID(eventID)
}
