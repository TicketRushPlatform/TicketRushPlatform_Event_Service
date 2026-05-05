package handler

import (
	"errors"
	"event_service/internal/apperror"
	"event_service/internal/dto"
	"event_service/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EventHandler struct {
	service services.EventService
	logger  *zap.Logger
}

func NewEventHandler(service services.EventService, logger *zap.Logger) *EventHandler {
	return &EventHandler{
		service: service,
		logger:  logger,
	}
}

func (h *EventHandler) RegisterRoutes(rg *gin.RouterGroup) {
	events := rg.Group("/events")
	{
		events.POST("", h.CreateEvent)
		events.GET("", h.ListEvents)
		events.GET("/:id", h.GetEvent)
		events.GET("/:id/showtimes", h.ListShowtimesByEvent)
		events.PUT("/:id/showtimes", h.ReplaceEventShowtimes)
		events.PUT("/:id", h.UpdateEvent)
		events.DELETE("/:id", h.DeleteEvent)
	}

	showtimes := rg.Group("/showtimes")
	{
		showtimes.GET("/:id", h.GetShowtime)
	}
}

// CreateEvent godoc
// @Summary Create event
// @Description Create a new event or movie.
// @Tags events
// @Accept json
// @Produce json
// @Param request body dto.CreateEventRequest true "Create event request"
// @Success 201 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events [post]
func (h *EventHandler) CreateEvent(c *gin.Context) {
	var req dto.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	event, err := h.service.CreateEvent(req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.SuccessResponse{
		Message: "event created successfully",
		Data:    event,
	})
}

// GetEvent godoc
// @Summary Get event by ID
// @Description Retrieve event detail by ID.
// @Tags events
// @Produce json
// @Param id path string true "Event ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events/{id} [get]
func (h *EventHandler) GetEvent(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid event ID",
		})
		return
	}

	event, err := h.service.GetEvent(eventID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.SuccessResponse{Data: event})
}

// ListEvents godoc
// @Summary List events
// @Description List events with pagination and optional filters.
// @Tags events
// @Produce json
// @Param page query int false "Page" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param type query string false "Event type"
// @Param search query string false "Search by name"
// @Success 200 {object} dto.PaginatedResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events [get]
func (h *EventHandler) ListEvents(c *gin.Context) {
	var query dto.ListEventsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid query params: " + err.Error(),
		})
		return
	}

	events, total, totalPages, err := h.service.ListEvents(query)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data:       events,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}

// UpdateEvent godoc
// @Summary Update event
// @Description Update event by ID.
// @Tags events
// @Accept json
// @Produce json
// @Param id path string true "Event ID"
// @Param request body dto.UpdateEventRequest true "Update event request"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events/{id} [put]
func (h *EventHandler) UpdateEvent(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid event ID",
		})
		return
	}

	var req dto.UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	event, err := h.service.UpdateEvent(eventID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "event updated successfully",
		Data:    event,
	})
}

// DeleteEvent godoc
// @Summary Delete event
// @Description Soft-delete event by ID.
// @Tags events
// @Produce json
// @Param id path string true "Event ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events/{id} [delete]
func (h *EventHandler) DeleteEvent(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid event ID",
		})
		return
	}

	if err := h.service.DeleteEvent(eventID); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "event deleted successfully",
	})
}

func (h *EventHandler) GetShowtime(c *gin.Context) {
	showtimeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid showtime ID",
		})
		return
	}
	showtime, err := h.service.GetShowtime(showtimeID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{Data: showtime})
}

func (h *EventHandler) ListShowtimesByEvent(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid event ID",
		})
		return
	}
	showtimes, err := h.service.ListShowtimesByEvent(eventID)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{Data: showtimes})
}

func (h *EventHandler) ReplaceEventShowtimes(c *gin.Context) {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid event ID",
		})
		return
	}
	var req []dto.UpsertShowtimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}
	showtimes, err := h.service.ReplaceShowtimesByEvent(eventID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse{
		Message: "event showtimes replaced successfully",
		Data:    showtimes,
	})
}

func (h *EventHandler) handleError(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.Code, dto.ErrorResponse{
			Code:    appErr.Code,
			Message: appErr.Message,
		})
		return
	}

	h.logger.Error("unhandled error", zap.Error(err))
	c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
		Code:    http.StatusInternalServerError,
		Message: "internal server error",
	})
}
