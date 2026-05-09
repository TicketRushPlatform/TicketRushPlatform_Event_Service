package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"event_service/internal/apperror"
	"event_service/internal/dto"
	"event_service/internal/middleware"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func bearerJWT(t *testing.T, secret, role string, sub uuid.UUID) string {
	t.Helper()
	claims := middleware.AuthClaims{
		Sub:  sub.String(),
		Role: role,
		Type: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

func TestEventHandler_AuthJWTAndRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "handler-jwt-secret"
	sub := uuid.New()
	now := time.Now().UTC()
	created := &dto.EventResponse{ID: uuid.NewString(), Name: "E", EventType: "MOVIE", CreatedAt: now, UpdatedAt: now}
	h := NewEventHandler(&eventServiceMock{
		createFn: func(req dto.CreateEventRequest) (*dto.EventResponse, error) {
			out := *created
			out.Name = req.Name
			return &out, nil
		},
		getFn:         func(uuid.UUID) (*dto.EventResponse, error) { return nil, errors.New("x") },
		getShowtimeFn: func(uuid.UUID) (*dto.ShowtimeResponse, error) { return nil, errors.New("x") },
		listShowtimesFn: func(uuid.UUID) ([]dto.ShowtimeResponse, error) {
			return nil, errors.New("x")
		},
		listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
			return nil, 0, 0, errors.New("x")
		},
		updateFn: func(uuid.UUID, dto.UpdateEventRequest) (*dto.EventResponse, error) { return nil, errors.New("x") },
		deleteFn: func(uuid.UUID) error { return errors.New("x") },
	}, zap.NewNop())

	r := gin.New()
	r.Use(middleware.RequireAuth(middleware.AuthConfig{
		JWTSecret: secret, JWTAlgorithm: "HS256",
	}))
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := mustJSON(t, dto.CreateEventRequest{Name: "E", DurationMinutes: 99, EventType: "MOVIE"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no bearer: status=%d body=%s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+bearerJWT(t, secret, "BOOKING_OWNER", sub))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("wrong role: status=%d body=%s", w2.Code, w2.Body.String())
	}

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Authorization", "Bearer "+bearerJWT(t, secret, "EVENT_OWNER", sub))
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusCreated {
		t.Fatalf("allowed role: status=%d body=%s", w3.Code, w3.Body.String())
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEventHandler_Create_Delete_AppErrorsThroughHandleError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := uuid.New()
	conflictMock := &eventServiceMock{
		createFn: func(req dto.CreateEventRequest) (*dto.EventResponse, error) {
			return nil, apperror.NewConflict("duplicate")
		},
		getFn:         func(uuid.UUID) (*dto.EventResponse, error) { return nil, errors.New("x") },
		getShowtimeFn: func(uuid.UUID) (*dto.ShowtimeResponse, error) { return nil, errors.New("x") },
		listShowtimesFn: func(uuid.UUID) ([]dto.ShowtimeResponse, error) {
			return nil, errors.New("x")
		},
		listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
			return nil, 0, 0, errors.New("x")
		},
		updateFn: func(uuid.UUID, dto.UpdateEventRequest) (*dto.EventResponse, error) { return nil, errors.New("x") },
		deleteFn: func(uuid.UUID) error { return apperror.NewNotFound("gone") },
	}
	h := NewEventHandler(conflictMock, zap.NewNop())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth_role", "ADMIN")
		c.Next()
	})
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader(mustJSON(t, dto.CreateEventRequest{Name: "E", DurationMinutes: 120, EventType: "MOVIE"})))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("create conflict: status=%d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/events/"+id.String(), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("delete not found: status=%d", w2.Code)
	}
}

func TestEventHandler_DeleteEvent_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewEventHandler(&eventServiceMock{
		createFn:        func(req dto.CreateEventRequest) (*dto.EventResponse, error) { return nil, errors.New("x") },
		getFn:           func(uuid.UUID) (*dto.EventResponse, error) { return nil, errors.New("x") },
		getShowtimeFn:   func(uuid.UUID) (*dto.ShowtimeResponse, error) { return nil, errors.New("x") },
		listShowtimesFn: func(uuid.UUID) ([]dto.ShowtimeResponse, error) { return nil, errors.New("x") },
		listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
			return nil, 0, 0, errors.New("x")
		},
		updateFn: func(uuid.UUID, dto.UpdateEventRequest) (*dto.EventResponse, error) { return nil, errors.New("x") },
		deleteFn: func(uuid.UUID) error { return errors.New("x") },
	}, zap.NewNop())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "not-uuid"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/x", nil)
	h.DeleteEvent(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", w.Code)
	}
}

func TestEventHandler_Create_ServiceGenericErrorUsesHandleErrorInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewEventHandler(&eventServiceMock{
		createFn: func(req dto.CreateEventRequest) (*dto.EventResponse, error) {
			return nil, errors.New("db")
		},
		getFn:           func(uuid.UUID) (*dto.EventResponse, error) { return nil, errors.New("x") },
		getShowtimeFn:   func(uuid.UUID) (*dto.ShowtimeResponse, error) { return nil, errors.New("x") },
		listShowtimesFn: func(uuid.UUID) ([]dto.ShowtimeResponse, error) { return nil, errors.New("x") },
		listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
			return nil, 0, 0, errors.New("x")
		},
		updateFn: func(uuid.UUID, dto.UpdateEventRequest) (*dto.EventResponse, error) { return nil, errors.New("x") },
		deleteFn: func(uuid.UUID) error { return errors.New("x") },
	}, zap.NewNop())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := mustJSON(t, dto.CreateEventRequest{Name: "E", DurationMinutes: 120, EventType: "MOVIE"})
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.CreateEvent(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", w.Code)
	}
}

type eventServiceMock struct {
	createFn           func(req dto.CreateEventRequest) (*dto.EventResponse, error)
	getFn              func(eventID uuid.UUID) (*dto.EventResponse, error)
	getShowtimeFn      func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error)
	listShowtimesFn    func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error)
	replaceShowtimesFn func(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error)
	listFn             func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error)
	updateFn           func(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error)
	deleteFn           func(eventID uuid.UUID) error
}

func (m *eventServiceMock) CreateEvent(req dto.CreateEventRequest) (*dto.EventResponse, error) {
	return m.createFn(req)
}
func (m *eventServiceMock) GetEvent(eventID uuid.UUID) (*dto.EventResponse, error) {
	return m.getFn(eventID)
}
func (m *eventServiceMock) ListEvents(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
	return m.listFn(query)
}
func (m *eventServiceMock) GetShowtime(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
	if m.getShowtimeFn == nil {
		return nil, nil
	}
	return m.getShowtimeFn(showtimeID)
}
func (m *eventServiceMock) ListShowtimesByEvent(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
	if m.listShowtimesFn == nil {
		return []dto.ShowtimeResponse{}, nil
	}
	return m.listShowtimesFn(eventID)
}
func (m *eventServiceMock) ReplaceShowtimesByEvent(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
	if m.replaceShowtimesFn == nil {
		return []dto.ShowtimeResponse{}, nil
	}
	return m.replaceShowtimesFn(eventID, showtimes)
}
func (m *eventServiceMock) UpdateEvent(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error) {
	return m.updateFn(eventID, req)
}
func (m *eventServiceMock) DeleteEvent(eventID uuid.UUID) error { return m.deleteFn(eventID) }

func TestEventHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := uuid.New()
	now := time.Now().UTC()
	res := &dto.EventResponse{ID: id.String(), Name: "Movie", EventType: "MOVIE", CreatedAt: now, UpdatedAt: now}
	showtimeRes := &dto.ShowtimeResponse{
		ID:          uuid.NewString(),
		EventID:     id.String(),
		Venue:       "Venue A",
		Address:     "Address A",
		StartTime:   now,
		EndTime:     now.Add(time.Hour),
		SeatMapName: "Map A",
	}

	mock := &eventServiceMock{
		createFn:      func(req dto.CreateEventRequest) (*dto.EventResponse, error) { return res, nil },
		getFn:         func(eventID uuid.UUID) (*dto.EventResponse, error) { return res, nil },
		getShowtimeFn: func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) { return showtimeRes, nil },
		listShowtimesFn: func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
			return []dto.ShowtimeResponse{*showtimeRes}, nil
		},
		listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
			return []dto.EventResponse{*res}, 1, 1, nil
		},
		updateFn: func(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error) { return res, nil },
		deleteFn: func(eventID uuid.UUID) error { return nil },
	}

	h := NewEventHandler(mock, zap.NewNop())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth_role", "ADMIN")
		c.Next()
	})
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1)

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{"create", http.MethodPost, "/api/v1/events", dto.CreateEventRequest{Name: "M", DurationMinutes: 100, EventType: "MOVIE"}, http.StatusCreated},
		{"list", http.MethodGet, "/api/v1/events?page=1&page_size=10", nil, http.StatusOK},
		{"get", http.MethodGet, "/api/v1/events/" + id.String(), nil, http.StatusOK},
		{"get showtime", http.MethodGet, "/api/v1/showtimes/" + showtimeRes.ID, nil, http.StatusOK},
		{"list showtimes by event", http.MethodGet, "/api/v1/events/" + id.String() + "/showtimes", nil, http.StatusOK},
		{"replace showtimes", http.MethodPut, "/api/v1/events/" + id.String() + "/showtimes", []dto.UpsertShowtimeRequest{{Venue: "Venue A", Address: "Address A", StartTime: now, EndTime: now.Add(time.Hour), SeatMapName: "Map A"}}, http.StatusOK},
		{"update", http.MethodPut, "/api/v1/events/" + id.String(), dto.UpdateEventRequest{Name: "U", DurationMinutes: 90, EventType: "EVENT"}, http.StatusOK},
		{"delete", http.MethodDelete, "/api/v1/events/" + id.String(), nil, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestEventHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		mock       *eventServiceMock
		wantStatus int
	}{
		{
			name:   "create invalid body",
			method: http.MethodPost, path: "/api/v1/events",
			body:       map[string]any{"name": "x"},
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "get invalid id",
			method: http.MethodGet, path: "/api/v1/events/invalid-uuid",
			mock: &eventServiceMock{
				getFn: func(eventID uuid.UUID) (*dto.EventResponse, error) { return nil, nil },
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "service app error",
			method: http.MethodGet, path: "/api/v1/events/" + uuid.New().String(),
			mock: &eventServiceMock{
				getFn: func(eventID uuid.UUID) (*dto.EventResponse, error) {
					return nil, apperror.NewNotFound("event not found")
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "service internal error",
			method: http.MethodDelete, path: "/api/v1/events/" + uuid.New().String(),
			mock: &eventServiceMock{
				deleteFn: func(eventID uuid.UUID) error { return errors.New("boom") },
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "showtime invalid id",
			method: http.MethodGet, path: "/api/v1/showtimes/invalid-id",
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "showtime service not found",
			method: http.MethodGet, path: "/api/v1/showtimes/" + uuid.New().String(),
			mock: &eventServiceMock{
				getShowtimeFn: func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
					return nil, apperror.NewNotFound("showtime not found")
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "event showtimes invalid event id",
			method: http.MethodGet, path: "/api/v1/events/bad-id/showtimes",
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "event showtimes internal error",
			method: http.MethodGet, path: "/api/v1/events/" + uuid.New().String() + "/showtimes",
			mock: &eventServiceMock{
				listShowtimesFn: func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
					return nil, errors.New("db boom")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "replace showtimes invalid event id",
			method:     http.MethodPut,
			path:       "/api/v1/events/bad-id/showtimes",
			body:       []dto.UpsertShowtimeRequest{{Venue: "V", Address: "A", StartTime: time.Now().UTC(), EndTime: time.Now().UTC().Add(time.Hour)}},
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "replace showtimes invalid body",
			method:     http.MethodPut,
			path:       "/api/v1/events/" + uuid.New().String() + "/showtimes",
			body:       map[string]any{"x": 1},
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "replace showtimes service app error",
			method: http.MethodPut,
			path:   "/api/v1/events/" + uuid.New().String() + "/showtimes",
			body:   []dto.UpsertShowtimeRequest{{Venue: "V", Address: "A", StartTime: time.Now().UTC(), EndTime: time.Now().UTC().Add(time.Hour)}},
			mock: &eventServiceMock{
				replaceShowtimesFn: func(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
					return nil, apperror.NewNotFound("event not found")
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:   "list invalid query",
			method: http.MethodGet, path: "/api/v1/events?page=0",
			mock: &eventServiceMock{
				listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
					return nil, 0, 0, nil
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "list service app error",
			method: http.MethodGet, path: "/api/v1/events?page=1&page_size=10",
			mock: &eventServiceMock{
				listFn: func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
					return nil, 0, 0, apperror.NewBadRequest("bad query")
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "update invalid id",
			method: http.MethodPut, path: "/api/v1/events/bad-id",
			body:       dto.UpdateEventRequest{Name: "x", DurationMinutes: 10, EventType: "EVENT"},
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "update invalid body",
			method: http.MethodPut, path: "/api/v1/events/" + uuid.New().String(),
			body:       map[string]any{"name": "x"},
			mock:       &eventServiceMock{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "update app error",
			method: http.MethodPut, path: "/api/v1/events/" + uuid.New().String(),
			body: dto.UpdateEventRequest{Name: "x", DurationMinutes: 10, EventType: "EVENT"},
			mock: &eventServiceMock{
				updateFn: func(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error) {
					return nil, apperror.NewNotFound("not found")
				},
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mock.createFn == nil {
				tt.mock.createFn = func(req dto.CreateEventRequest) (*dto.EventResponse, error) { return nil, nil }
			}
			if tt.mock.getFn == nil {
				tt.mock.getFn = func(eventID uuid.UUID) (*dto.EventResponse, error) { return nil, nil }
			}
			if tt.mock.listFn == nil {
				tt.mock.listFn = func(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) { return nil, 0, 0, nil }
			}
			if tt.mock.getShowtimeFn == nil {
				tt.mock.getShowtimeFn = func(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) { return nil, nil }
			}
			if tt.mock.listShowtimesFn == nil {
				tt.mock.listShowtimesFn = func(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) { return nil, nil }
			}
			if tt.mock.updateFn == nil {
				tt.mock.updateFn = func(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error) { return nil, nil }
			}
			if tt.mock.deleteFn == nil {
				tt.mock.deleteFn = func(eventID uuid.UUID) error { return nil }
			}

			h := NewEventHandler(tt.mock, zap.NewNop())
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("auth_role", "ADMIN")
				c.Next()
			})
			v1 := r.Group("/api/v1")
			h.RegisterRoutes(v1)

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
