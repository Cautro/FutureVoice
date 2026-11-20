package user

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func AdminListHandler(useCase *UserUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, err := parsePage(c.Query("page"), 1)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
			return
		}
		limit, err := parsePage(c.Query("limit"), 20)
		if err != nil || limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
		result, err := useCase.ListForAdmin(
			c.Request.Context(),
			c.GetInt64("user_id"),
			Role(c.Query("role")),
			c.Query("q"),
			page,
			limit,
		)
		if err != nil {
			writeAdminUserError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func AdminRoleHandler(useCase *UserUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || targetID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
			return
		}
		var input struct {
			Role Role `json:"role"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		if err := useCase.ChangeRole(c.Request.Context(), c.GetInt64("user_id"), targetID, input.Role); err != nil {
			writeAdminUserError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func AdminActiveHandler(useCase *UserUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || targetID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
			return
		}
		var input struct {
			Active *bool `json:"is_active"`
		}
		if err := c.ShouldBindJSON(&input); err != nil || input.Active == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "is_active must be provided as a boolean"})
			return
		}
		if err := useCase.ChangeActive(c.Request.Context(), c.GetInt64("user_id"), targetID, *input.Active); err != nil {
			writeAdminUserError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func parsePage(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("page must be positive")
	}
	return parsed, nil
}

func writeAdminUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
	case errors.Is(err, ErrLastAdmin):
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove or deactivate the last active administrator"})
	case errors.Is(err, ErrInvalidRole):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
	case errors.Is(err, ErrInvalidQuery):
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query must not exceed 100 characters"})
	case errors.Is(err, ErrSelfChange):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
