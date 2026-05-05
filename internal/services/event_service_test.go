package services

import (
	"errors"
	"event_service/internal/dto"
	"event_service/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type eventRepoMock struct {
	createFn     func(req dto.CreateEventRequest) (*models.Event, error)
	getByIDFn    func(eventID uuid.UUID) (*models.Event, error)
	listFn       func(query dto.ListEventsQuery) ([]models.Event, int64, error)
	getShowtimeFn func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error)
	listShowtimesFn func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error)
	replaceShowtimesFn func(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error)
	updateFn     func(eventID uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error)
	deleteFn     func(eventID uuid.UUID) error
	createCalls  int
	getByIDCalls int
	listCalls    int
	updateCalls  int
	deleteCalls  int
}

func (m *eventRepoMock) Create(req dto.CreateEventRequest) (*models.Event, error) {
	m.createCalls++
	return m.createFn(req)
}

func (m *eventRepoMock) GetByID(eventID uuid.UUID) (*models.Event, error) {
	m.getByIDCalls++
	return m.getByIDFn(eventID)
}

func (m *eventRepoMock) List(query dto.ListEventsQuery) ([]models.Event, int64, error) {
	m.listCalls++
	return m.listFn(query)
}

func (m *eventRepoMock) Update(eventID uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error) {
	m.updateCalls++
	return m.updateFn(eventID, req)
}

func (m *eventRepoMock) GetShowtimeByID(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
	if m.getShowtimeFn == nil {
		return nil, nil
	}
	return m.getShowtimeFn(showtimeID)
}

func (m *eventRepoMock) ListShowtimesByEventID(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
	if m.listShowtimesFn == nil {
		return []dto.ShowtimeResponse{}, nil
	}
	return m.listShowtimesFn(eventID)
}

func (m *eventRepoMock) ReplaceShowtimesByEventID(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
	if m.replaceShowtimesFn == nil {
		return []dto.ShowtimeResponse{}, nil
	}
	return m.replaceShowtimesFn(eventID, showtimes)
}

func (m *eventRepoMock) Delete(eventID uuid.UUID) error {
	m.deleteCalls++
	return m.deleteFn(eventID)
}

func sampleEvent() models.Event {
	now := time.Now().UTC()
	director := "Nolan"
	rating := "PG-13"
	language := "EN"
	release := now.Add(-24 * time.Hour)

	return models.Event{
		Base: models.Base{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:            "Interstellar",
		Description:     "Sci-fi movie",
		DurationMinutes: 169,
		EventType:       models.EventTypeMovie,
		Director:        &director,
		AgeRating:       &rating,
		ReleaseDate:     &release,
		Language:        &language,
	}
}

func TestEventService_AllMethods(t *testing.T) {
	repoErr := errors.New("repo error")
	event := sampleEvent()
	eventID := uuid.New()

	tests := []struct {
		name string
		run  func(t *testing.T, svc EventService, mock *eventRepoMock)
	}{
		{
			name: "create event success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.createFn = func(req dto.CreateEventRequest) (*models.Event, error) {
					return &event, nil
				}
				got, err := svc.CreateEvent(dto.CreateEventRequest{
					Name:            "Interstellar",
					DurationMinutes: 169,
					EventType:       "MOVIE",
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ID == "" || got.Name != "Interstellar" {
					t.Fatalf("unexpected create response: %+v", got)
				}

				mock.createFn = func(req dto.CreateEventRequest) (*models.Event, error) {
					return nil, repoErr
				}
				_, err = svc.CreateEvent(dto.CreateEventRequest{
					Name:            "Fail case",
					DurationMinutes: 100,
					EventType:       "EVENT",
				})
				if !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
		{
			name: "get event success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.getByIDFn = func(id uuid.UUID) (*models.Event, error) {
					return &event, nil
				}
				got, err := svc.GetEvent(eventID)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ID == "" {
					t.Fatalf("expected mapped event response")
				}

				mock.getByIDFn = func(id uuid.UUID) (*models.Event, error) {
					return nil, repoErr
				}
				_, err = svc.GetEvent(eventID)
				if !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
		{
			name: "list events success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.listFn = func(query dto.ListEventsQuery) ([]models.Event, int64, error) {
					return []models.Event{event}, 3, nil
				}
				got, total, pages, err := svc.ListEvents(dto.ListEventsQuery{
					Page:     1,
					PageSize: 2,
					Type:     "MOVIE",
					Search:   "Inter",
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 1 || total != 3 || pages != 2 {
					t.Fatalf("unexpected list result len=%d total=%d pages=%d", len(got), total, pages)
				}

				mock.listFn = func(query dto.ListEventsQuery) ([]models.Event, int64, error) {
					return nil, 0, repoErr
				}
				_, _, _, err = svc.ListEvents(dto.ListEventsQuery{Page: 1, PageSize: 10})
				if !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
		{
			name: "update event success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.updateFn = func(id uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error) {
					return &event, nil
				}
				got, err := svc.UpdateEvent(eventID, dto.UpdateEventRequest{
					Name:            "Updated",
					Description:     "Updated desc",
					DurationMinutes: 120,
					EventType:       "EVENT",
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ID == "" {
					t.Fatalf("expected mapped response")
				}

				mock.updateFn = func(id uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error) {
					return nil, repoErr
				}
				_, err = svc.UpdateEvent(eventID, dto.UpdateEventRequest{
					Name:            "Updated",
					DurationMinutes: 120,
					EventType:       "EVENT",
				})
				if !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
		{
			name: "delete event success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.deleteFn = func(id uuid.UUID) error {
					return nil
				}
				if err := svc.DeleteEvent(eventID); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				mock.deleteFn = func(id uuid.UUID) error {
					return repoErr
				}
				if err := svc.DeleteEvent(eventID); !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
		{
			name: "get showtime success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.getShowtimeFn = func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
					return &dto.ShowtimeResponse{ID: showtimeID.String(), EventID: eventID.String(), Venue: "Venue A", Address: "Address A"}, nil
				}
				got, err := svc.GetShowtime(eventID)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == nil || got.ID == "" {
					t.Fatalf("expected showtime payload")
				}

				mock.getShowtimeFn = func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
					return nil, repoErr
				}
				_, err = svc.GetShowtime(eventID)
				if !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
		{
			name: "list showtimes by event success and error",
			run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
				mock.listShowtimesFn = func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
					return []dto.ShowtimeResponse{
						{ID: uuid.NewString(), EventID: eventID.String(), Venue: "Venue A", Address: "Address A"},
						{ID: uuid.NewString(), EventID: eventID.String(), Venue: "Venue B", Address: "Address B"},
					}, nil
				}
				got, err := svc.ListShowtimesByEvent(eventID)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 2 {
					t.Fatalf("expected 2 showtimes, got %d", len(got))
				}

				mock.listShowtimesFn = func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
					return nil, repoErr
				}
				_, err = svc.ListShowtimesByEvent(eventID)
				if !errors.Is(err, repoErr) {
					t.Fatalf("expected %v, got %v", repoErr, err)
				}
			},
		},
			{
				name: "replace showtimes success and error",
				run: func(t *testing.T, svc EventService, mock *eventRepoMock) {
					mock.replaceShowtimesFn = func(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
						return []dto.ShowtimeResponse{{ID: uuid.NewString(), EventID: eventID.String(), Venue: "Venue A", Address: "Addr A"}}, nil
					}
					got, err := svc.ReplaceShowtimesByEvent(eventID, []dto.UpsertShowtimeRequest{{
						Venue:     "Venue A",
						Address:   "Addr A",
						StartTime: time.Now().UTC(),
						EndTime:   time.Now().UTC().Add(time.Hour),
					}})
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if len(got) != 1 {
						t.Fatalf("expected 1 showtime, got %d", len(got))
					}

					mock.replaceShowtimesFn = func(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
						return nil, repoErr
					}
					_, err = svc.ReplaceShowtimesByEvent(eventID, nil)
					if !errors.Is(err, repoErr) {
						t.Fatalf("expected %v, got %v", repoErr, err)
					}
				},
			},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &eventRepoMock{
				createFn: func(req dto.CreateEventRequest) (*models.Event, error) {
					e := sampleEvent()
					return &e, nil
				},
				getByIDFn: func(eventID uuid.UUID) (*models.Event, error) {
					e := sampleEvent()
					return &e, nil
				},
				listFn: func(query dto.ListEventsQuery) ([]models.Event, int64, error) {
					e := sampleEvent()
					return []models.Event{e}, 1, nil
				},
				updateFn: func(eventID uuid.UUID, req dto.UpdateEventRequest) (*models.Event, error) {
					e := sampleEvent()
					return &e, nil
				},
				deleteFn: func(eventID uuid.UUID) error {
					return nil
				},
			}

			svc := NewEventService(zap.NewNop(), mock)
			tt.run(t, svc, mock)
		})
	}
}
