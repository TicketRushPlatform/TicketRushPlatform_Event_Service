package services

import (
	"event_service/internal/dto"
	"event_service/internal/repository"
	"math"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EventService interface {
	CreateEvent(req dto.CreateEventRequest) (*dto.EventResponse, error)
	GetEvent(eventID uuid.UUID) (*dto.EventResponse, error)
	ListEvents(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error)
	GetShowtime(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error)
	ListShowtimesByEvent(eventID uuid.UUID) ([]dto.ShowtimeResponse, error)
	ReplaceShowtimesByEvent(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error)
	UpdateEvent(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error)
	DeleteEvent(eventID uuid.UUID) error
}

type eventService struct {
	logger     *zap.Logger
	repository repository.EventRepository
}

func NewEventService(logger *zap.Logger, repository repository.EventRepository) EventService {
	return &eventService{
		logger:     logger,
		repository: repository,
	}
}

func (s *eventService) CreateEvent(req dto.CreateEventRequest) (*dto.EventResponse, error) {
	s.logger.Info("creating event", zap.String("name", req.Name), zap.String("type", req.EventType))

	event, err := s.repository.Create(req)
	if err != nil {
		return nil, err
	}

	return event.ToDTO(), nil
}

func (s *eventService) GetEvent(eventID uuid.UUID) (*dto.EventResponse, error) {
	s.logger.Debug("getting event", zap.String("event_id", eventID.String()))

	event, err := s.repository.GetByID(eventID)
	if err != nil {
		return nil, err
	}

	return event.ToDTO(), nil
}

func (s *eventService) ListEvents(query dto.ListEventsQuery) ([]dto.EventResponse, int64, int, error) {
	s.logger.Debug("listing events",
		zap.Int("page", query.Page),
		zap.Int("page_size", query.PageSize),
		zap.String("type", query.Type),
		zap.String("search", query.Search),
	)

	events, total, err := s.repository.List(query)
	if err != nil {
		return nil, 0, 0, err
	}

	responses := make([]dto.EventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, *event.ToDTO())
	}

	totalPages := int(math.Ceil(float64(total) / float64(query.PageSize)))

	return responses, total, totalPages, nil
}

func (s *eventService) GetShowtime(showtimeID uuid.UUID) (*dto.ShowtimeResponse, error) {
	s.logger.Debug("getting showtime", zap.String("showtime_id", showtimeID.String()))
	return s.repository.GetShowtimeByID(showtimeID)
}

func (s *eventService) ListShowtimesByEvent(eventID uuid.UUID) ([]dto.ShowtimeResponse, error) {
	s.logger.Debug("listing event showtimes", zap.String("event_id", eventID.String()))
	return s.repository.ListShowtimesByEventID(eventID)
}

func (s *eventService) ReplaceShowtimesByEvent(eventID uuid.UUID, showtimes []dto.UpsertShowtimeRequest) ([]dto.ShowtimeResponse, error) {
	s.logger.Info("replacing event showtimes", zap.String("event_id", eventID.String()), zap.Int("count", len(showtimes)))
	return s.repository.ReplaceShowtimesByEventID(eventID, showtimes)
}

func (s *eventService) UpdateEvent(eventID uuid.UUID, req dto.UpdateEventRequest) (*dto.EventResponse, error) {
	s.logger.Info("updating event", zap.String("event_id", eventID.String()))

	event, err := s.repository.Update(eventID, req)
	if err != nil {
		return nil, err
	}

	return event.ToDTO(), nil
}

func (s *eventService) DeleteEvent(eventID uuid.UUID) error {
	s.logger.Info("deleting event", zap.String("event_id", eventID.String()))
	return s.repository.Delete(eventID)
}
