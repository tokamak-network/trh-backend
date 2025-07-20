package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

type HealthHandler struct{}

// ShowAccount godoc
//
//	@Summary		Get health
//	@Description	Get health
//	@Tags			health
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	entities.Response
//	@Router			/health [get]
func (h *HealthHandler) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, entities.Response{
		Status:  http.StatusOK,
		Message: "OK",
	})
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}
