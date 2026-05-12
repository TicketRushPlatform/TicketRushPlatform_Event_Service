package repository

import (
	"event_service/internal/apperror"
	"event_service/internal/dto"
	"event_service/internal/models"
	"fmt"
	"strings"
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
	ListSeatMaps() ([]dto.SeatMapResponse, error)
	CreateSeatMap(req dto.CreateSeatMapRequest) (*dto.SeatMapResponse, error)
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
	creatorID, err := uuid.Parse(req.CreatorID)
	if err != nil {
		return nil, apperror.NewInternal("invalid creator id", err)
	}

	event := &models.Event{
		CreatorID:            creatorID,
		Name:                 req.Name,
		Description:          req.Description,
		DurationMinutes:      req.DurationMinutes,
		EventType:            models.EventType(req.EventType),
		Category:             req.Category,
		Venue:                req.Venue,
		City:                 req.City,
		Address:              req.Address,
		Organizer:            req.Organizer,
		ImageURL:             req.ImageURL,
		SaleOpensAt:          req.SaleOpensAt,
		IsFlashSale:          req.IsFlashSale,
		Status:               req.Status,
		Director:             req.Director,
		AgeRating:            req.AgeRating,
		ReleaseDate:          req.ReleaseDate,
		Language:             req.Language,
		MaxTicketsPerBooking: req.MaxTicketsPerBooking,
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
		"name":                    req.Name,
		"description":             req.Description,
		"duration_minutes":        req.DurationMinutes,
		"event_type":              req.EventType,
		"category":                req.Category,
		"venue":                   req.Venue,
		"city":                    req.City,
		"address":                 req.Address,
		"organizer":               req.Organizer,
		"image_url":               req.ImageURL,
		"sale_opens_at":           req.SaleOpensAt,
		"is_flash_sale":           req.IsFlashSale,
		"status":                  req.Status,
		"director":                req.Director,
		"age_rating":              req.AgeRating,
		"release_date":            req.ReleaseDate,
		"language":                req.Language,
		"max_tickets_per_booking": req.MaxTicketsPerBooking,
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
			sm.name AS seat_map_name,
			st.queue_enabled,
			st.queue_limit
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
			sm.name AS seat_map_name,
			st.queue_enabled,
			st.queue_limit
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

func (r *eventRepository) ListSeatMaps() ([]dto.SeatMapResponse, error) {
	seatMaps := make([]dto.SeatMapResponse, 0)
	err := r.db.Raw(`
		SELECT
			sm.id::text AS id,
			sm.name,
			sm.venue_id::text AS venue_id,
			v.name AS venue_name,
			v.address AS venue_address
		FROM seat_maps sm
		INNER JOIN venues v ON v.id = sm.venue_id
		WHERE sm.deleted_at IS NULL
		ORDER BY sm.name
	`).Scan(&seatMaps).Error
	if err != nil {
		return nil, apperror.NewInternal("failed to list seat maps", err)
	}
	if err := r.loadSeatMapSeats(seatMaps); err != nil {
		return nil, err
	}
	return seatMaps, nil
}

func (r *eventRepository) CreateSeatMap(req dto.CreateSeatMapRequest) (*dto.SeatMapResponse, error) {
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
	venueID := uuid.New()
	seatMapID := uuid.New()

	if err := tx.Exec(
		`INSERT INTO venues (id, name, address, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		venueID, strings.TrimSpace(req.Venue), strings.TrimSpace(req.Address), now, now,
	).Error; err != nil {
		tx.Rollback()
		return nil, apperror.NewInternal("failed to create venue for seat map", err)
	}

	if err := tx.Exec(
		`INSERT INTO seat_maps (id, name, venue_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		seatMapID, strings.TrimSpace(req.Name), venueID, now, now,
	).Error; err != nil {
		tx.Rollback()
		return nil, apperror.NewInternal("failed to create seat map", err)
	}

	for _, seat := range req.Seats {
		if err := tx.Exec(
			`INSERT INTO seats (id, seat_map_id, row, number, seat_class, default_price, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New(), seatMapID, strings.TrimSpace(seat.Row), seat.Number, seat.SeatClass, seat.Price, now, now,
		).Error; err != nil {
			tx.Rollback()
			return nil, apperror.NewInternal("failed to create seat", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, apperror.NewInternal("failed to commit seat map", err)
	}

	seatMap, err := r.getSeatMapByID(seatMapID)
	if err != nil {
		return nil, err
	}
	return seatMap, nil
}

func (r *eventRepository) getSeatMapByID(seatMapID uuid.UUID) (*dto.SeatMapResponse, error) {
	var seatMap dto.SeatMapResponse
	err := r.db.Raw(`
		SELECT
			sm.id::text AS id,
			sm.name,
			sm.venue_id::text AS venue_id,
			v.name AS venue_name,
			v.address AS venue_address
		FROM seat_maps sm
		INNER JOIN venues v ON v.id = sm.venue_id
		WHERE sm.id = ? AND sm.deleted_at IS NULL
		LIMIT 1
	`, seatMapID).Scan(&seatMap).Error
	if err != nil {
		return nil, apperror.NewInternal("failed to get seat map", err)
	}
	if seatMap.ID == "" {
		return nil, apperror.NewNotFound("seat map not found")
	}
	seatMaps := []dto.SeatMapResponse{seatMap}
	if err := r.loadSeatMapSeats(seatMaps); err != nil {
		return nil, err
	}
	return &seatMaps[0], nil
}

func (r *eventRepository) loadSeatMapSeats(seatMaps []dto.SeatMapResponse) error {
	if len(seatMaps) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(seatMaps))
	indexByID := make(map[string]int, len(seatMaps))
	for index := range seatMaps {
		id, err := uuid.Parse(seatMaps[index].ID)
		if err != nil {
			return apperror.NewInternal("invalid seat map id", err)
		}
		ids = append(ids, id)
		indexByID[seatMaps[index].ID] = index
	}

	type seatRow struct {
		SeatMapID string
		dto.SeatMapSeatResponse
	}
	rows := make([]seatRow, 0)
	err := r.db.Raw(`
		SELECT
			seat_map_id::text AS seat_map_id,
			id::text AS id,
			row,
			number,
			seat_class,
			COALESCE(default_price, CASE seat_class
				WHEN 'DELUXE' THEN 420000
				WHEN 'PREMIUM' THEN 320000
				WHEN 'VIP' THEN 250000
				ELSE 180000
			END)::float AS price
		FROM seats
		WHERE seat_map_id IN ? AND deleted_at IS NULL
		ORDER BY row, number
	`, ids).Scan(&rows).Error
	if err != nil {
		return apperror.NewInternal("failed to list seat map seats", err)
	}

	for _, row := range rows {
		if index, ok := indexByID[row.SeatMapID]; ok {
			seatMaps[index].Seats = append(seatMaps[index].Seats, row.SeatMapSeatResponse)
		}
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

	if len(showtimes) == 0 {
		if err := tx.Exec(`UPDATE show_times SET deleted_at = ?, updated_at = ? WHERE event_id = ? AND deleted_at IS NULL`, now, now, eventID).Error; err != nil {
			tx.Rollback()
			return nil, apperror.NewInternal("failed to clear showtimes", err)
		}
		if err := tx.Commit().Error; err != nil {
			return nil, apperror.NewInternal("failed to commit showtime updates", err)
		}
		return r.ListShowtimesByEventID(eventID)
	}

	finalIDs := make([]uuid.UUID, 0, len(showtimes))

	for index, item := range showtimes {
		seatMapName := item.SeatMapName
		if seatMapName == "" {
			seatMapName = fmt.Sprintf("Auto map %d", index+1)
		}
		queueLimit := item.QueueLimit
		if queueLimit <= 0 {
			queueLimit = 50
		}
		if queueLimit > 10000 {
			queueLimit = 10000
		}

		var existingID *uuid.UUID
		if item.ID != nil && strings.TrimSpace(*item.ID) != "" {
			if id, err := uuid.Parse(strings.TrimSpace(*item.ID)); err == nil {
				var cnt int64
				if err := tx.Raw(`SELECT COUNT(*) FROM show_times WHERE id = ? AND event_id = ? AND deleted_at IS NULL`, id, eventID).Scan(&cnt).Error; err != nil {
					tx.Rollback()
					return nil, apperror.NewInternal("failed to verify showtime", err)
				}
				if cnt == 1 {
					existingID = &id
				}
			}
		}

		if existingID != nil {
			var currentSeatMapIDStr string
			var currentSeatMapID uuid.UUID
			if err := tx.Raw("SELECT seat_map_id FROM show_times WHERE id = ? AND event_id = ? AND deleted_at IS NULL", *existingID, eventID).Scan(&currentSeatMapIDStr).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to fetch current seat map", err)
			}
			currentSeatMapID, _ = uuid.Parse(currentSeatMapIDStr)

			var targetSeatMapIDStr string
			var targetSeatMapID uuid.UUID
			if err := tx.Raw(`SELECT id FROM seat_maps WHERE name = ? AND deleted_at IS NULL LIMIT 1`, seatMapName).Scan(&targetSeatMapIDStr).Error; err != nil || targetSeatMapIDStr == "" {
				// Fallback: we cannot easily safely create a new seat map here without seats.
				// Just keep the current one or error. Given current UI, name should always be found.
				targetSeatMapID = currentSeatMapID
			} else {
				targetSeatMapID, _ = uuid.Parse(targetSeatMapIDStr)
			}

			if err := tx.Exec(`
				UPDATE show_times 
				SET start_time = ?, end_time = ?, queue_enabled = ?, queue_limit = ?, seat_map_id = ?, updated_at = ?
				WHERE id = ? AND event_id = ? AND deleted_at IS NULL
			`, item.StartTime, item.EndTime, item.QueueEnabled, queueLimit, targetSeatMapID, now, *existingID, eventID).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to update showtime", err)
			}

			// Also update venue via the target map's venue_id
			if err := tx.Exec(`
				UPDATE venues v
				SET name = ?, address = ?, updated_at = ?
				FROM seat_maps sm
				WHERE sm.id = ? AND v.id = sm.venue_id
			`, item.Venue, item.Address, now, targetSeatMapID).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to update venue for showtime", err)
			}

			// If seat map changed, we need to recreate show_time_seats and seat_pricing
			if targetSeatMapID != currentSeatMapID {
				if err := tx.Exec("DELETE FROM seat_pricing WHERE show_time_id = ?", *existingID).Error; err != nil {
					tx.Rollback()
					return nil, apperror.NewInternal("failed to delete old seat prices", err)
				}
				if err := tx.Exec("DELETE FROM show_time_seats WHERE show_time_id = ?", *existingID).Error; err != nil {
					tx.Rollback()
					return nil, apperror.NewInternal("failed to delete old show_time_seats", err)
				}

				if err := tx.Exec(`
					INSERT INTO show_time_seats (show_time_id, seat_id, status, created_at, updated_at)
					SELECT ?, id, 'AVAILABLE', ?, ?
					FROM seats
					WHERE seat_map_id = ?
				`, *existingID, now, now, targetSeatMapID).Error; err != nil {
					tx.Rollback()
					return nil, apperror.NewInternal("failed to generate new seats", err)
				}

				if err := tx.Exec(`
					INSERT INTO seat_pricing (show_time_id, seat_id, price, created_at, updated_at)
					SELECT ?, id, 
						COALESCE(default_price, CASE seat_class 
							WHEN 'DELUXE' THEN 420000 
							WHEN 'PREMIUM' THEN 320000 
							WHEN 'VIP' THEN 250000 
							ELSE 180000 
						END), ?, ?
					FROM seats
					WHERE seat_map_id = ?
				`, *existingID, now, now, targetSeatMapID).Error; err != nil {
					tx.Rollback()
					return nil, apperror.NewInternal("failed to generate new seat pricing", err)
				}
			} else {
				if err := tx.Exec(`
					UPDATE seat_pricing sp
					SET price = COALESCE(s.default_price, CASE s.seat_class
							WHEN 'DELUXE' THEN 420000
							WHEN 'PREMIUM' THEN 320000
							WHEN 'VIP' THEN 250000
							ELSE 180000
						END),
						updated_at = ?
					FROM seats s
					WHERE sp.show_time_id = ? AND sp.seat_id = s.id AND s.seat_map_id = ?
				`, now, *existingID, targetSeatMapID).Error; err != nil {
					tx.Rollback()
					return nil, apperror.NewInternal("failed to refresh seat pricing", err)
				}
			}

			finalIDs = append(finalIDs, *existingID)
			continue
		}

		showtimeID := uuid.New()

		var existingSeatMapID *uuid.UUID
		var existingVenueID *uuid.UUID

		var sm struct {
			ID      uuid.UUID
			VenueID uuid.UUID
		}
		if err := tx.Raw(`SELECT id, venue_id FROM seat_maps WHERE name = ? AND deleted_at IS NULL LIMIT 1`, seatMapName).Scan(&sm).Error; err == nil && sm.ID != uuid.Nil {
			existingSeatMapID = &sm.ID
			existingVenueID = &sm.VenueID
		}

		if existingSeatMapID != nil {
			// Reuse existing seat map and update its venue
			if err := tx.Exec(`
				UPDATE venues
				SET name = ?, address = ?, updated_at = ?
				WHERE id = ?
			`, item.Venue, item.Address, now, *existingVenueID).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to update existing venue", err)
			}

			if err := tx.Exec(
				`INSERT INTO show_times (id, event_id, seat_map_id, start_time, end_time, queue_enabled, queue_limit, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				showtimeID, eventID, *existingSeatMapID, item.StartTime, item.EndTime, item.QueueEnabled, queueLimit, now, now,
			).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to create showtime with existing map", err)
			}

			// Generate show_time_seats for the new showtime
			if err := tx.Exec(`
				INSERT INTO show_time_seats (show_time_id, seat_id, status, created_at, updated_at)
				SELECT ?, id, 'AVAILABLE', ?, ?
				FROM seats
				WHERE seat_map_id = ?
			`, showtimeID, now, now, *existingSeatMapID).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to generate seats for showtime", err)
			}

			// Generate seat_pricing for the new showtime
			if err := tx.Exec(`
				INSERT INTO seat_pricing (show_time_id, seat_id, price, created_at, updated_at)
				SELECT ?, id, 
					COALESCE(default_price, CASE seat_class 
						WHEN 'DELUXE' THEN 420000 
						WHEN 'PREMIUM' THEN 320000 
						WHEN 'VIP' THEN 250000 
						ELSE 180000 
					END), ?, ?
				FROM seats
				WHERE seat_map_id = ?
			`, showtimeID, now, now, *existingSeatMapID).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to generate seat pricing for showtime", err)
			}
		} else {
			// Create new venue and map
			venueID := uuid.New()
			seatMapID := uuid.New()

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
				`INSERT INTO show_times (id, event_id, seat_map_id, start_time, end_time, queue_enabled, queue_limit, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				showtimeID, eventID, seatMapID, item.StartTime, item.EndTime, item.QueueEnabled, queueLimit, now, now,
			).Error; err != nil {
				tx.Rollback()
				return nil, apperror.NewInternal("failed to create showtime", err)
			}
		}

		finalIDs = append(finalIDs, showtimeID)
	}

	orphanQ := tx.Table("show_times").Where("event_id = ? AND deleted_at IS NULL", eventID)
	if len(finalIDs) > 0 {
		orphanQ = orphanQ.Where("id NOT IN ?", finalIDs)
	}
	if err := orphanQ.Updates(map[string]interface{}{
		"deleted_at": now,
		"updated_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, apperror.NewInternal("failed to remove old showtimes", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, apperror.NewInternal("failed to commit showtime updates", err)
	}
	return r.ListShowtimesByEventID(eventID)
}
