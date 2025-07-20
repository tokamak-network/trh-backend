package thanos

import (
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// @Summary		Get All Stacks
// @Description	Get All Stacks (Authenticated users)
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	entities.Response
// @Failure		401	{object}	map[string]interface{}
// @Router			/stacks/thanos [get]
func (h *ThanosDeploymentHandler) GetAllStacks(c *gin.Context) {
	response, err := h.ThanosDeploymentService.GetAllStacks()
	if err != nil {
		logger.Error("failed to get all stacks", zap.Error(err))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Get Stack Status
// @Description	Get Stack Status
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/status [get]
func (h *ThanosDeploymentHandler) GetStackStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackStatus(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get stack status", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Get Stack By ID
// @Description	Get Stack By ID
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id} [get]
func (h *ThanosDeploymentHandler) GetStackByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetStackByID(uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to get stack by id", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}
