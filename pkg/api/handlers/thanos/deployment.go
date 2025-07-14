package thanos

import (
	"net/http"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// @Summary      Deploy Thanos Stack
// @Description  Deploy Thanos Stack (Admin only)
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      dtos.DeployThanosRequest  true  "Deploy Thanos Stack Request"
// @Success      200      {object}  entities.Response
// @Failure      401      {object}  map[string]interface{}
// @Failure      403      {object}  map[string]interface{}
// @Router       /stacks/thanos [post]
func (h *ThanosDeploymentHandler) Deploy(c *gin.Context) {
	var request dtos.DeployThanosRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	if err := request.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	if request.RegisterCandidate {
		if request.RegisterCandidateParams == nil {
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "registerCandidateParams is required",
				Data:    nil,
			})
			return
		}

		if err := request.RegisterCandidateParams.Validate(c.Request.Context()); err != nil {
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
	} else {
		request.RegisterCandidateParams = nil
	}

	request.AdminAccount = utils.TrimPrivateKey(request.AdminAccount)
	request.SequencerAccount = utils.TrimPrivateKey(request.SequencerAccount)
	request.BatcherAccount = utils.TrimPrivateKey(request.BatcherAccount)
	request.ProposerAccount = utils.TrimPrivateKey(request.ProposerAccount)

	response, err := h.ThanosDeploymentService.CreateThanosStack(c, request)
	if err != nil {
		logger.Error("failed to deploy thanos stack", zap.Error(err))
	}

	c.JSON(int(response.Status), response)
}

// @Summary      Stop Thanos Stack
// @Description  Stop Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id}/stop [post]
func (h *ThanosDeploymentHandler) Stop(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.StopDeployingThanosStack(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to stop thanos stack", zap.Error(err))
	}
	c.JSON(int(response.Status), response)
}

// @Summary      Resume Thanos Stack
// @Description  Resume Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id}/resume [post]
func (h *ThanosDeploymentHandler) Resume(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.ResumeThanosStack(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to resume thanos stack", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary      Terminate Thanos Stack
// @Description  Terminate Thanos Stack
// @Tags         Thanos Stack
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Thanos Stack ID"
// @Success      200      {object}  entities.Response
// @Router       /stacks/thanos/{id} [delete]
func (h *ThanosDeploymentHandler) Terminate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.TerminateThanosStack(c, uuid.MustParse(id))
	if err != nil {
		logger.Error("failed to terminate thanos stack", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}
