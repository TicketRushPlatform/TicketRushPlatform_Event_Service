package repository

import (
	"errors"
	"event_service/internal/dto"
	"event_service/internal/models"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupEventRepo(t *testing.T) EventRepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Event{}, &models.EventReview{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewEventRepository(db, zap.NewNop())
}

func setupEventRepoMockDB(t *testing.T) (EventRepository, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	return NewEventRepository(gdb, zap.NewNop()), mock
}

func TestEventRepository_CRUD(t *testing.T) {
	repo := setupEventRepo(t)
	trailerURL := "https://www.youtube.com/embed/example"

	createReq := dto.CreateEventRequest{
		CreatorID:       uuid.NewString(),
		Name:            "Cinema Night",
		Description:     "Movie",
		DurationMinutes: 120,
		EventType:       "MOVIE",
		TrailerURL:      &trailerURL,
	}

	created, err := repo.Create(createReq)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Name != createReq.Name {
		t.Fatalf("GetByID() wrong data")
	}
	if got.TrailerURL == nil || *got.TrailerURL != trailerURL {
		t.Fatalf("GetByID() expected trailer URL to be persisted, got %+v", got.TrailerURL)
	}

	events, total, err := repo.List(dto.ListEventsQuery{
		Page:     1,
		PageSize: 10,
		Type:     "MOVIE",
	})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(events) != 1 || total != 1 {
		t.Fatalf("List() unexpected len=%d total=%d", len(events), total)
	}

	updated, err := repo.Update(created.ID, dto.UpdateEventRequest{
		Name:            "Cinema Night Updated",
		Description:     "Movie Updated",
		DurationMinutes: 130,
		EventType:       "EVENT",
	})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Name != "Cinema Night Updated" || string(updated.EventType) != "EVENT" {
		t.Fatalf("Update() not applied")
	}

	review, err := repo.CreateReview(created.ID, dto.CreateEventReviewRequest{
		UserID:     uuid.NewString(),
		AuthorName: "Minh Anh",
		Rating:     5,
		Comment:    "Great event.",
	})
	if err != nil {
		t.Fatalf("CreateReview() error: %v", err)
	}
	if review.EventID != created.ID || review.Rating != 5 {
		t.Fatalf("CreateReview() wrong data: %+v", review)
	}

	reviews, err := repo.ListReviewsByEventID(created.ID)
	if err != nil {
		t.Fatalf("ListReviewsByEventID() error: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Comment != "Great event." {
		t.Fatalf("ListReviewsByEventID() unexpected reviews: %+v", reviews)
	}

	if err := repo.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := repo.GetByID(created.ID); err == nil {
		t.Fatalf("expected not found after delete")
	}
}

func TestEventRepository_NotFoundPaths(t *testing.T) {
	repo := setupEventRepo(t)
	id := uuid.New()

	if _, err := repo.GetByID(id); err == nil {
		t.Fatalf("expected not found")
	}
	if _, err := repo.Update(id, dto.UpdateEventRequest{
		Name:            "x",
		DurationMinutes: 10,
		EventType:       "EVENT",
	}); err == nil {
		t.Fatalf("expected update not found")
	}
	if err := repo.Delete(id); err == nil {
		t.Fatalf("expected delete not found")
	}
}

func TestEventRepository_ErrorPathsWithMock(t *testing.T) {
	repo, mock := setupEventRepoMockDB(t)
	eventID := uuid.New()

	t.Run("create invalid creator id", func(t *testing.T) {
		_, err := repo.Create(dto.CreateEventRequest{
			CreatorID:       "not-a-uuid",
			Name:            "x",
			DurationMinutes: 1,
			EventType:       "EVENT",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("create internal error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "events"`).
			WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		_, err := repo.Create(dto.CreateEventRequest{
			CreatorID:       uuid.NewString(),
			Name:            "x",
			DurationMinutes: 1,
			EventType:       "EVENT",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("get by id internal error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .* FROM "events"`).
			WillReturnError(errors.New("db down"))
		_, err := repo.GetByID(eventID)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("list count error via search", func(t *testing.T) {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "events"`).
			WillReturnError(errors.New("bad where"))

		_, _, err := repo.List(dto.ListEventsQuery{
			Page:     1,
			PageSize: 10,
			Search:   "abc",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("delete internal error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "events" SET "deleted_at"`).
			WillReturnError(errors.New("delete failed"))
		mock.ExpectRollback()

		err := repo.Delete(eventID)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("list find error without search", func(t *testing.T) {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "events"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
		mock.ExpectQuery(`SELECT .* FROM "events"`).
			WillReturnError(errors.New("find failed"))

		_, _, err := repo.List(dto.ListEventsQuery{
			Page:     1,
			PageSize: 10,
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	expectSingleEventFirst := func(mock sqlmock.Sqlmock, eid uuid.UUID) {
		rows := sqlmock.NewRows([]string{"id", "name", "description", "duration_minutes", "event_type", "created_at", "updated_at", "deleted_at"}).
			AddRow(eid, "Event A", "Desc", 120, "EVENT", time.Now().UTC(), time.Now().UTC(), nil)
		mock.ExpectQuery(`SELECT .* FROM "events"`).
			WithArgs(eid, 1).
			WillReturnRows(rows)
	}

	t.Run("update model updates error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectSingleEventFirst(mock, eventID)
		mock.ExpectExec(`UPDATE`).
			WillReturnError(errors.New("update failed"))

		_, err := repo.Update(eventID, dto.UpdateEventRequest{
			Name: "n", DurationMinutes: 1, EventType: "EVENT",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("update reload first error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectSingleEventFirst(mock, eventID)
		mock.ExpectExec(`UPDATE`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`SELECT .* FROM "events"`).
			WithArgs(eventID, 1).
			WillReturnError(errors.New("reload failed"))

		_, err := repo.Update(eventID, dto.UpdateEventRequest{
			Name: "n", DurationMinutes: 1, EventType: "EVENT",
		})
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestEventRepository_ShowtimeQueries_WithMock(t *testing.T) {
	repo, mock := setupEventRepoMockDB(t)
	showtimeID := uuid.New()
	eventID := uuid.New()
	now := time.Now().UTC()

	t.Run("get showtime success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}).
			AddRow(showtimeID.String(), eventID.String(), "Venue A", "Address A", now, now.Add(time.Hour), "Map A", false, 50)
		mock.ExpectQuery(`SELECT`).
			WithArgs(showtimeID).
			WillReturnRows(rows)
		got, err := repo.GetShowtimeByID(showtimeID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.ID != showtimeID.String() {
			t.Fatalf("unexpected showtime payload: %+v", got)
		}
	})

	t.Run("get showtime not found", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"})
		mock.ExpectQuery(`SELECT`).
			WithArgs(showtimeID).
			WillReturnRows(rows)
		_, err := repo.GetShowtimeByID(showtimeID)
		if err == nil {
			t.Fatalf("expected not found error")
		}
	})

	t.Run("get showtime internal error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT`).
			WithArgs(showtimeID).
			WillReturnError(errors.New("query failed"))
		_, err := repo.GetShowtimeByID(showtimeID)
		if err == nil {
			t.Fatalf("expected query error")
		}
	})

	t.Run("list showtimes by event success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}).
			AddRow(uuid.NewString(), eventID.String(), "Venue A", "Address A", now, now.Add(time.Hour), "Map A", false, 50).
			AddRow(uuid.NewString(), eventID.String(), "Venue B", "Address B", now.Add(2*time.Hour), now.Add(3*time.Hour), "Map B", true, 1)
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnRows(rows)
		got, err := repo.ListShowtimesByEventID(eventID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 showtimes, got %d", len(got))
		}
	})

	t.Run("list showtimes by event error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnError(fmt.Errorf("query failed"))
		_, err := repo.ListShowtimesByEventID(eventID)
		if err == nil {
			t.Fatalf("expected query error")
		}
	})
}

func TestEventRepository_SeatMaps_WithMock(t *testing.T) {
	t.Run("list seat maps success loads seats", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mapID := uuid.New()
		venueID := uuid.New()
		seatID := uuid.New()

		mock.ExpectQuery(`SELECT`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "venue_id", "venue_name", "venue_address"}).
				AddRow(mapID.String(), "Main hall", venueID.String(), "Venue A", "Addr A"))
		mock.ExpectQuery(`SELECT`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"seat_map_id", "id", "row", "number", "seat_class", "price"}).
				AddRow(mapID.String(), seatID.String(), "A", 1, "VIP", 250000))

		got, err := repo.ListSeatMaps()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || len(got[0].Seats) != 1 || got[0].Seats[0].ID != seatID.String() {
			t.Fatalf("unexpected seat maps: %+v", got)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("list seat maps query error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("query failed"))

		if _, err := repo.ListSeatMaps(); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("list seat maps rejects invalid uuid before loading seats", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectQuery(`SELECT`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "venue_id", "venue_name", "venue_address"}).
				AddRow("bad-id", "Main hall", uuid.NewString(), "Venue A", "Addr A"))

		if _, err := repo.ListSeatMaps(); err == nil {
			t.Fatalf("expected invalid uuid error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("load seat map seats handles empty input", func(t *testing.T) {
		repo, _ := setupEventRepoMockDB(t)
		if err := repo.(*eventRepository).loadSeatMapSeats(nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("load seat map seats query error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("seats query failed"))

		err := repo.(*eventRepository).loadSeatMapSeats([]dto.SeatMapResponse{{ID: uuid.NewString()}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("get seat map by id not found", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		seatMapID := uuid.New()
		mock.ExpectQuery(`SELECT`).
			WithArgs(seatMapID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "venue_id", "venue_name", "venue_address"}))

		if _, err := repo.(*eventRepository).getSeatMapByID(seatMapID); err == nil {
			t.Fatalf("expected not found")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("get seat map by id query error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		seatMapID := uuid.New()
		mock.ExpectQuery(`SELECT`).
			WithArgs(seatMapID).
			WillReturnError(errors.New("seat map query failed"))

		if _, err := repo.(*eventRepository).getSeatMapByID(seatMapID); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestEventRepository_CreateSeatMap_WithMock(t *testing.T) {
	successReq := dto.CreateSeatMapRequest{
		Name:    " Main hall ",
		Venue:   " Venue A ",
		Address: " Addr A ",
		Seats: []dto.CreateSeatMapSeatDTO{
			{Row: " A ", Number: 1, SeatClass: "VIP", Price: 250000},
			{Row: "A", Number: 2, SeatClass: "STANDARD", Price: 180000},
		},
	}

	t.Run("success trims fields and reloads created map", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mapID := uuid.New()
		venueID := uuid.New()

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO venues`).
			WithArgs(sqlmock.AnyArg(), "Venue A", "Addr A", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).
			WithArgs(sqlmock.AnyArg(), "Main hall", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seats`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "A", 1, "VIP", float64(250000), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seats`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "A", 2, "STANDARD", float64(180000), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectQuery(`SELECT`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "venue_id", "venue_name", "venue_address"}).
				AddRow(mapID.String(), "Main hall", venueID.String(), "Venue A", "Addr A"))
		mock.ExpectQuery(`SELECT`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"seat_map_id", "id", "row", "number", "seat_class", "price"}).
				AddRow(mapID.String(), uuid.NewString(), "A", 1, "VIP", 250000).
				AddRow(mapID.String(), uuid.NewString(), "A", 2, "STANDARD", 180000))

		got, err := repo.CreateSeatMap(successReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "Main hall" || len(got.Seats) != 2 {
			t.Fatalf("unexpected seat map: %+v", got)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		if _, err := repo.CreateSeatMap(successReq); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("venue insert error rolls back", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO venues`).WillReturnError(errors.New("venue failed"))
		mock.ExpectRollback()

		if _, err := repo.CreateSeatMap(successReq); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("seat map insert error rolls back", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO venues`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).WillReturnError(errors.New("seat map failed"))
		mock.ExpectRollback()

		if _, err := repo.CreateSeatMap(successReq); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("seat insert error rolls back", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO venues`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seats`).WillReturnError(errors.New("seat failed"))
		mock.ExpectRollback()

		if _, err := repo.CreateSeatMap(successReq); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		req := successReq
		req.Seats = req.Seats[:1]
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO venues`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seats`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		if _, err := repo.CreateSeatMap(req); err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}

func TestEventRepository_ReplaceShowtimesByEventID_WithMock(t *testing.T) {
	eventID := uuid.New()
	showtimeID := uuid.New()
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(2 * time.Hour)

	expectGetEvent := func(mock sqlmock.Sqlmock) {
		rows := sqlmock.NewRows([]string{"id", "name", "description", "duration_minutes", "event_type", "created_at", "updated_at", "deleted_at"}).
			AddRow(eventID, "Event A", "Desc", 120, "EVENT", time.Now().UTC(), time.Now().UTC(), nil)
		mock.ExpectQuery(`SELECT .* FROM "events"`).
			WithArgs(eventID, 1).
			WillReturnRows(rows)
	}

	t.Run("success with auto seat map name", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()

		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).
			WithArgs("Auto map 1").
			WillReturnError(gorm.ErrRecordNotFound)

		mock.ExpectExec(`INSERT INTO venues`).
			WithArgs(sqlmock.AnyArg(), "Venue A", "Addr A", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).
			WithArgs(sqlmock.AnyArg(), "Auto map 1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_times`).
			WithArgs(sqlmock.AnyArg(), eventID, sqlmock.AnyArg(), start, end, false, 50, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectExec(`UPDATE "show_times"`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectCommit()

		rows := sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}).
			AddRow(showtimeID.String(), eventID.String(), "Venue A", "Addr A", start, end, "Auto map 1", false, 50)
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnRows(rows)

		got, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue:     "Venue A",
			Address:   "Addr A",
			StartTime: start,
			EndTime:   end,
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 showtime, got %d", len(got))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("begin transaction error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "Venue A", Address: "Addr A", StartTime: start, EndTime: end,
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("orphan soft-delete error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).
			WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectExec(`INSERT INTO venues`).
			WithArgs(sqlmock.AnyArg(), "Venue A", "Addr A", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_times`).
			WithArgs(sqlmock.AnyArg(), eventID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE "show_times"`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID, sqlmock.AnyArg()).
			WillReturnError(errors.New("update failed"))
		mock.ExpectRollback()

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "Venue A", Address: "Addr A", StartTime: start, EndTime: end,
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("insert venue error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectExec(`INSERT INTO venues`).
			WillReturnError(errors.New("insert venue failed"))
		mock.ExpectRollback()

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "Venue A", Address: "Addr A", StartTime: start, EndTime: end,
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("commit error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectExec(`INSERT INTO venues`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_times`).
			WithArgs(sqlmock.AnyArg(), eventID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), false, 50, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE "show_times"`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "Venue A", Address: "Addr A", StartTime: start, EndTime: end,
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("replace event not found", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		mock.ExpectQuery(`SELECT .* FROM "events"`).
			WithArgs(eventID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "V", Address: "A", StartTime: start, EndTime: end,
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("insert seat map error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectExec(`INSERT INTO venues`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).
			WillReturnError(errors.New("seat map failed"))
		mock.ExpectRollback()

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "Venue A", Address: "Addr A", StartTime: start, EndTime: end, SeatMapName: "M",
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("insert show time error", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectExec(`INSERT INTO venues`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_maps`).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_times`).
			WillReturnError(errors.New("showtime insert failed"))
		mock.ExpectRollback()

		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue: "Venue A", Address: "Addr A", StartTime: start, EndTime: end, SeatMapName: "M",
		}})
		if err == nil {
			t.Fatalf("expected error")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("updates in place when id belongs to event", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()

		stID := showtimeID.String()
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(int64(1))
		mock.ExpectQuery(`SELECT COUNT`).
			WithArgs(showtimeID, eventID).
			WillReturnRows(countRows)

		mock.ExpectQuery(`SELECT seat_map_id FROM show_times`).
			WithArgs(showtimeID, eventID).
			WillReturnRows(sqlmock.NewRows([]string{"seat_map_id"}).AddRow(uuid.New()))

		mock.ExpectQuery(`SELECT id FROM seat_maps`).
			WithArgs("Map A").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

		mock.ExpectExec(`UPDATE show_times`).
			WithArgs(start, end, true, 99, sqlmock.AnyArg(), sqlmock.AnyArg(), showtimeID, eventID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE venues v`).
			WithArgs("Venue A", "Addr A", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`DELETE FROM seat_pricing`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`DELETE FROM show_time_seats`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_time_seats`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO seat_pricing`).WillReturnResult(sqlmock.NewResult(0, 1))

		mock.ExpectExec(`UPDATE "show_times"`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID, showtimeID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectCommit()

		listRows := sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}).
			AddRow(stID, eventID.String(), "Venue A", "Addr A", start, end, "Map A", true, 99)
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnRows(listRows)

		idStr := stID
		_, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			ID:           &idStr,
			Venue:        "Venue A",
			Address:      "Addr A",
			StartTime:    start,
			EndTime:      end,
			SeatMapName:  "Map A",
			QueueEnabled: true,
			QueueLimit:   99,
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("empty input clears showtimes", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE show_times SET deleted_at`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID).
			WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectCommit()
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}))

		got, err := repo.ReplaceShowtimesByEventID(eventID, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected no showtimes, got %d", len(got))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("creates showtime with existing seat map", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		seatMapID := uuid.New()
		venueID := uuid.New()
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT id, venue_id FROM seat_maps`).
			WithArgs("Map A").
			WillReturnRows(sqlmock.NewRows([]string{"id", "venue_id"}).AddRow(seatMapID, venueID))
		mock.ExpectExec(`UPDATE venues`).
			WithArgs("Venue A", "Addr A", sqlmock.AnyArg(), venueID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_times`).
			WithArgs(sqlmock.AnyArg(), eventID, seatMapID, start, end, true, 10000, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO show_time_seats`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), seatMapID).
			WillReturnResult(sqlmock.NewResult(0, 4))
		mock.ExpectExec(`INSERT INTO seat_pricing`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), seatMapID).
			WillReturnResult(sqlmock.NewResult(0, 4))
		mock.ExpectExec(`UPDATE "show_times"`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}).
				AddRow(uuid.NewString(), eventID.String(), "Venue A", "Addr A", start, end, "Map A", true, 10000))

		got, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			Venue:        "Venue A",
			Address:      "Addr A",
			StartTime:    start,
			EndTime:      end,
			SeatMapName:  "Map A",
			QueueEnabled: true,
			QueueLimit:   20000,
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].QueueLimit != 10000 {
			t.Fatalf("unexpected showtimes: %+v", got)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})

	t.Run("updates existing showtime and refreshes pricing when map is unchanged", func(t *testing.T) {
		repo, mock := setupEventRepoMockDB(t)
		expectGetEvent(mock)
		currentSeatMapID := uuid.New()
		idStr := showtimeID.String()
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT COUNT`).
			WithArgs(showtimeID, eventID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
		mock.ExpectQuery(`SELECT seat_map_id FROM show_times`).
			WithArgs(showtimeID, eventID).
			WillReturnRows(sqlmock.NewRows([]string{"seat_map_id"}).AddRow(currentSeatMapID.String()))
		mock.ExpectQuery(`SELECT id FROM seat_maps`).
			WithArgs("Missing map").
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectExec(`UPDATE show_times`).
			WithArgs(start, end, false, 50, currentSeatMapID, sqlmock.AnyArg(), showtimeID, eventID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE venues v`).
			WithArgs("Venue A", "Addr A", sqlmock.AnyArg(), currentSeatMapID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE seat_pricing sp`).
			WithArgs(sqlmock.AnyArg(), showtimeID, currentSeatMapID).
			WillReturnResult(sqlmock.NewResult(0, 4))
		mock.ExpectExec(`UPDATE "show_times"`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), eventID, showtimeID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()
		mock.ExpectQuery(`SELECT`).
			WithArgs(eventID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "venue", "address", "start_time", "end_time", "seat_map_name", "queue_enabled", "queue_limit"}).
				AddRow(idStr, eventID.String(), "Venue A", "Addr A", start, end, "Missing map", false, 50))

		got, err := repo.ReplaceShowtimesByEventID(eventID, []dto.UpsertShowtimeRequest{{
			ID:          &idStr,
			Venue:       "Venue A",
			Address:     "Addr A",
			StartTime:   start,
			EndTime:     end,
			SeatMapName: "Missing map",
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].QueueLimit != 50 {
			t.Fatalf("unexpected showtimes: %+v", got)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations: %v", err)
		}
	})
}
