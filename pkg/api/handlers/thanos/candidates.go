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

// @Summary		Register Candidates
// @Description	Register Candidates
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/register-candidates [post]
func (h *ThanosDeploymentHandler) RegisterCandidates(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
	}

	var request dtos.RegisterCandidateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	if err := request.Validate(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	response, err := h.ThanosDeploymentService.RegisterCandidate(c, uuid.MustParse(id), request)
	if err != nil {
		logger.Error("failed to register candidate", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}
