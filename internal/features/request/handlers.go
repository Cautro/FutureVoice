package request

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreateHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CreateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}

		created, err := useCase.Create(c.Request.Context(), input)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, created)
	}
}

func ListHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		telegramID, err := parsePositiveInt64(c.Query("telegram_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id must be a positive integer"})
			return
		}

		scope := c.Query("scope")
		if scope != "" && scope != "mine" && scope != "all" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be mine or all"})
			return
		}

		limit := 0
		if rawLimit := c.Query("limit"); rawLimit != "" {
			limit, err = strconv.Atoi(rawLimit)
			if err != nil || limit < 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
				return
			}
		}

		var requests []Request
		if scope == "all" {
			requests, err = useCase.ListAll(c.Request.Context(), telegramID, limit)
		} else {
			requests, err = useCase.ListMine(c.Request.Context(), telegramID, limit)
		}
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, requests)
	}
}

func StudentGetHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		telegramID, err := parsePositiveInt64(c.Query("telegram_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id must be a positive integer"})
			return
		}
		item, err := useCase.GetMine(c.Request.Context(), telegramID, requestID)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func DeleteHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		telegramID, err := parsePositiveInt64(c.Query("telegram_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id must be a positive integer"})
			return
		}

		if err := useCase.Delete(c.Request.Context(), telegramID, requestID); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func UpdateStatusHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}

		var input StatusUpdateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		updated, err := useCase.UpdateStatus(c.Request.Context(), input.TelegramID, requestID, input.Status)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

// AdminListHandler exposes a paginated request queue to authenticated staff.
// Authentication and role checks are provided by the router middleware; the
// use case still verifies the persisted user and role before querying data.
func AdminListHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := Status(c.Query("status"))
		requestType := Type(c.Query("type"))
		page, err := parseOptionalPositiveInt(c.Query("page"), 1)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a positive integer"})
			return
		}
		limit, err := parseOptionalPositiveInt(c.Query("limit"), 20)
		if err != nil || limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}

		result, err := useCase.ListForPanel(c.Request.Context(), c.GetInt64("user_id"), AdminFilter{
			Status: status,
			Type:   requestType,
			Query:  c.Query("q"),
			Page:   page,
			Limit:  limit,
		})
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func AdminGetHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		result, err := useCase.GetForPanel(c.Request.Context(), c.GetInt64("user_id"), requestID)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func AdminUpdateStatusHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		var input struct {
			Status Status `json:"status"`
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		updated, err := useCase.UpdateStatusFromPanel(
			c.Request.Context(),
			c.GetInt64("user_id"),
			requestID,
			input.Status,
			input.Reason,
		)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

func AdminAssignHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		var input map[string]json.RawMessage
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		rawAssignee, ok := input["assigned_to"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "assigned_to must be provided (a user ID or null)"})
			return
		}
		var assignedTo *int64
		if string(rawAssignee) != "null" {
			var id int64
			if err := json.Unmarshal(rawAssignee, &id); err != nil || id <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "assigned_to must be a positive integer or null"})
				return
			}
			assignedTo = &id
		}
		if err := useCase.Assign(
			c.Request.Context(),
			c.GetInt64("user_id"),
			requestID,
			assignedTo,
		); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func AdminCommentHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		var input struct {
			Body string `json:"body"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		if err := useCase.AddInternalComment(
			c.Request.Context(),
			c.GetInt64("user_id"),
			requestID,
			input.Body,
		); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusCreated)
	}
}

func AdminMessageHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		var input struct {
			Body       string `json:"body"`
			Visibility string `json:"visibility"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		if input.Visibility == "" {
			input.Visibility = "internal"
		}
		if err := useCase.AddStaffMessage(c.Request.Context(), c.GetInt64("user_id"), requestID, input.Body, input.Visibility); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusCreated)
	}
}

func StudentMessagesHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		telegramID, err := parsePositiveInt64(c.Query("telegram_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id must be a positive integer"})
			return
		}
		messages, err := useCase.ListMessagesForStudent(c.Request.Context(), telegramID, requestID)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, messages)
	}
}

func StudentAddMessageHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		var input struct {
			TelegramID int64  `json:"telegram_id"`
			Body       string `json:"body"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		if err := useCase.AddMessageFromStudent(c.Request.Context(), input.TelegramID, requestID, input.Body); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusCreated)
	}
}

func TelegramNotificationsHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, err := parseOptionalPositiveInt(c.Query("limit"), 20)
		if err != nil || limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
		items, err := useCase.PendingTelegramNotifications(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load notifications"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func TelegramNotificationDeliveredHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "notification ID must be a positive integer"})
			return
		}
		if err := useCase.MarkTelegramNotificationDelivered(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not acknowledge notification"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func TelegramNotificationFailedHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "notification ID must be a positive integer"})
			return
		}
		var input struct {
			Error string `json:"error"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		if err := useCase.FailTelegramNotification(c.Request.Context(), id, input.Error); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not reschedule notification"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func AdminDeleteHandler(useCase *RequestUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, err := parsePositiveInt64(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request ID must be a positive integer"})
			return
		}
		if err := useCase.DeleteFromPanel(
			c.Request.Context(),
			c.GetInt64("user_id"),
			requestID,
		); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func parseOptionalPositiveInt(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("value must be positive")
	}
	return parsed, nil
}

func parsePositiveInt64(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("value must be positive")
	}
	return parsed, nil
}

func writeUseCaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrInvalidType), errors.Is(err, ErrInvalidAssignee):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "request cannot be changed in its current state"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
