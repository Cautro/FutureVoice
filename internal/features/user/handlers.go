package user

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterTelegramUserHandler(useCase *UserUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var profile TelegramProfile
		if err := c.ShouldBindJSON(&profile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
			return
		}
		if profile.TelegramID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id must be positive"})
			return
		}

		profile.Username = strings.TrimSpace(profile.Username)
		profile.FirstName = strings.TrimSpace(profile.FirstName)
		profile.LastName = strings.TrimSpace(profile.LastName)

		registered, err := useCase.GetOrCreateTelegramUser(c.Request.Context(), profile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register user"})
			return
		}
		c.JSON(http.StatusOK, registered)
	}
}
