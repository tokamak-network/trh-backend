package thanos

import (
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// @Summary		Update Network
// @Description	Update Network
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Thanos Stack ID"
// @Param			request	body		dtos.UpdateNetworkRequest	true	"Update Network Request"
// @Success		200		{object}	entities.Response
// @Router			/stacks/thanos/{id} [put]
func (h *ThanosDeploymentHandler) UpdateNetwork(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	var request dtos.UpdateNetworkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UpdateNetwork(c, uuid.MustParse(id), request)
	if err != nil {
		logger.Error("failed to update network", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}
