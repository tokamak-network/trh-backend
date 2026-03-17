package thanos

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// @Summary		Deploy Thanos Stack
// @Description	Deploy Thanos Stack (Admin only)
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		dtos.DeployThanosRequest	true	"Deploy Thanos Stack Request"
// @Success		200		{object}	entities.Response
// @Failure		401		{object}	map[string]interface{}
// @Failure		403		{object}	map[string]interface{}
// @Router			/stacks/thanos [post]
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

	// Validate preset ID when provided.
	if request.PresetID != "" {
		presetSvc := presets.NewService()
		if _, err := presetSvc.GetByID(request.PresetID); err != nil {
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}
	}

	if request.FeeToken != "" {
		upperToken := strings.ToUpper(request.FeeToken)
		request.FeeToken = upperToken
		validTokens := []string{"TON", "ETH", "USDT", "USDC"}
		valid := false
		for _, t := range validTokens {
			if upperToken == t {
				valid = true
				break
			}
		}
		if !valid {
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  http.StatusBadRequest,
				Message: fmt.Sprintf("invalid feeToken: %s. Must be one of: TON, ETH, USDT, USDC", upperToken),
				Data:    nil,
			})
			return
		}
	}

	// Sanitize seed phrase — clear it from the request after use so it is never
	// written to logs or persisted in raw form.
	request.SeedPhrase = ""

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

// @Summary		Stop Thanos Stack
// @Description	Stop Thanos Stack
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/stop [post]
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

// @Summary		Resume Thanos Stack
// @Description	Resume Thanos Stack
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/resume [post]
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

// @Summary		Terminate Thanos Stack
// @Description	Terminate Thanos Stack
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id} [delete]
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

// @Summary     Get Deployment Logs
// @Description Get logs for a deployment (paginated)
// @Tags        Thanos Stack
// @Accept      json
// @Produce     json
// @Param       id path string true "Thanos Stack ID"
// @Param       deploymentId path string true "Deployment ID"
// @Param       limit query int false "Max logs to return" default(200)
// @Param       afterId query string false "Return logs after this log id (exclusive)"
// @Success     200 {object} entities.Response
// @Router      /stacks/thanos/{id}/deployments/{deploymentId}/logs [get]
func (h *ThanosDeploymentHandler) GetDeploymentLogs(c *gin.Context) {
	id := c.Param("id")
	deploymentId := c.Param("deploymentId")

	if id == "" || deploymentId == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{Status: http.StatusBadRequest, Message: "id and deploymentId are required"})
		return
	}

	limit := 200
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	var afterIDPtr *string
	if after := c.Query("afterId"); after != "" {
		afterIDPtr = &after
	}

	resp, err := h.ThanosDeploymentService.GetDeploymentLogs(uuid.MustParse(id), uuid.MustParse(deploymentId), limit, afterIDPtr)
	if err != nil {
		logger.Error("failed to get deployment logs", zap.Error(err))
	}
	c.JSON(int(resp.Status), resp)
}

// @Summary     Get Stack Logs
// @Description Get logs across all deployments for a stack (paginated)
// @Tags        Thanos Stack
// @Accept      json
// @Produce     json
// @Param       id path string true "Thanos Stack ID"
// @Param       limit query int false "Max logs to return" default(200)
// @Param       afterId query string false "Return logs after this log id (exclusive)"
// @Success     200 {object} entities.Response
// @Router      /stacks/thanos/{id}/logs [get]
func (h *ThanosDeploymentHandler) GetStackLogs(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{Status: http.StatusBadRequest, Message: "id is required"})
		return
	}

	limit := 200
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	var afterIDPtr *string
	if after := c.Query("afterId"); after != "" {
		afterIDPtr = &after
	}

	resp, err := h.ThanosDeploymentService.GetStackLogs(uuid.MustParse(id), limit, afterIDPtr)
	if err != nil {
		logger.Error("failed to get stack logs", zap.Error(err))
	}
	c.JSON(int(resp.Status), resp)
}

// @Summary     Download Deployment Log File
// @Description Download the deployment log file
// @Tags        Thanos Stack
// @Accept      json
// @Produce     application/octet-stream
// @Param       id path string true "Thanos Stack ID"
// @Param       deploymentId path string true "Deployment ID"
// @Success     200 {file} file "Log file content"
// @Failure     400 {object} entities.Response
// @Failure     404 {object} entities.Response
// @Failure     500 {object} entities.Response
// @Router      /stacks/thanos/{id}/deployments/{deploymentId}/logs/download [get]
func (h *ThanosDeploymentHandler) DownloadDeploymentLogFile(c *gin.Context) {
	stackIdStr := c.Param("id")
	deploymentIdStr := c.Param("deploymentId")

	if stackIdStr == "" || deploymentIdStr == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id and deploymentId are required",
			Data:    nil,
		})
		return
	}

	stackId, err := uuid.Parse(stackIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack ID format",
			Data:    nil,
		})
		return
	}

	deploymentId, err := uuid.Parse(deploymentIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid deployment ID format",
			Data:    nil,
		})
		return
	}

	// Get deployment and validate log file
	deployment, err := h.ThanosDeploymentService.DownloadDeploymentLogFile(stackId, deploymentId)
	if err != nil {
		logger.Error("failed to get deployment log file",
			zap.String("stackId", stackIdStr),
			zap.String("deploymentId", deploymentIdStr),
			zap.Error(err))

		var statusCode int
		if err.Error() == "stack not found" || err.Error() == "deployment not found" {
			statusCode = http.StatusNotFound
		} else {
			statusCode = http.StatusInternalServerError
		}

		c.JSON(statusCode, &entities.Response{
			Status:  uint64(statusCode),
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	// Generate filename based on deployment step and ID
	filename := fmt.Sprintf("deployment_%s_%s.log", deployment.Step, deployment.ID.String()[:8])

	// Prepare file for download
	result, err := utils.PrepareFileDownload(c.Request.Context(), utils.FileDownloadConfig{
		FilePath:    deployment.LogPath,
		Filename:    filename,
		ContentType: "application/octet-stream",
	})
	if err != nil {
		logger.Error("failed to prepare file download", zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "failed to prepare file for download",
			Data:    nil,
		})
		return
	}
	defer result.File.Close()

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", result.Filename))
	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", result.Size))

	// Stream the file content
	_, err = io.Copy(c.Writer, result.File)
	if err != nil {
		logger.Error("failed to stream file", zap.Error(err))
		// At this point headers are already sent, so we can't send a JSON error response
		return
	}
}

// @Summary		Validate Deployment
// @Description	Validate deployment parameters and check pre-requisites
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			request	body		dtos.ValidateDeploymentRequest	true	"Validate Deployment Request"
// @Success		200		{object}	entities.Response
// @Failure		400		{object}	entities.Response
// @Router			/stacks/thanos/validate-deployment [post]
func (h *ThanosDeploymentHandler) Validate(c *gin.Context) {
	var request dtos.ValidateDeploymentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.ValidateDeployment(c.Request.Context(), &request)
	if err != nil {
		logger.Error("failed to validate deployment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  http.StatusOK,
		Message: "Validation completed",
		Data:    response,
	})
}
