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

// @Summary		Get Integrations
// @Description	Get Integrations
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations [get]
func (h *ThanosDeploymentHandler) GetIntegrations(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetIntegrations(stackUUID)
	if err != nil {
		logger.Error("failed to get integrations", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Get Integration By ID
// @Description	Get Integration By ID
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id				path		string	true	"Thanos Stack ID"
// @Param			integrationId	path		string	true	"Integration ID"
// @Success		200				{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/{integrationId} [get]
func (h *ThanosDeploymentHandler) GetIntegrationById(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	integrationId := c.Param("integrationId")
	if integrationId == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "integrationId is required",
			Data:    nil,
		})
		return
	}
	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}
	integrationUUID, err := uuid.Parse(integrationId)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid integration id format",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.GetIntegration(
		stackUUID,
		integrationUUID,
	)
	if err != nil {
		logger.Error("failed to get integration", zap.Error(err), zap.String("id", id), zap.String("integrationId", integrationId))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Install Bridge
// @Description	Install Bridge
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/bridge [post]
func (h *ThanosDeploymentHandler) InstallBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallBridge(c, id)
	if err != nil {
		logger.Error("failed to install bridge", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Uninstall Bridge
// @Description	Uninstall Bridge
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/bridge [delete]
func (h *ThanosDeploymentHandler) UninstallBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallBridge(c, id)
	if err != nil {
		logger.Error("failed to uninstall bridge", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Install Block Explorer
// @Description	Install Block Explorer
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id		path		string								true	"Thanos Stack ID"
// @Param			request	body		dtos.InstallBlockExplorerRequest	true	"Install Block Explorer Request"
// @Success		200		{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/block-explorer [post]
func (h *ThanosDeploymentHandler) InstallBlockExplorer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.InstallBlockExplorerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallBlockExplorer(c, id, request)
	if err != nil {
		logger.Error("failed to install block explorer", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Uninstall Block Explorer
// @Description	Uninstall Block Explorer
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/block-explorer [delete]
func (h *ThanosDeploymentHandler) UninstallBlockExplorer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallBlockExplorer(c, id)
	if err != nil {
		logger.Error("failed to uninstall block explorer", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Install Monitoring
// @Description	Install Monitoring
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id		path		string							true	"Thanos Stack ID"
// @Param			request	body		dtos.InstallMonitoringRequest	true	"Install Monitoring Request"
// @Success		200		{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/monitoring [post]
func (h *ThanosDeploymentHandler) InstallMonitoring(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.InstallMonitoringRequest
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
	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallMonitoring(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to install monitoring", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Uninstall Monitoring
// @Description	Uninstall Monitoring
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/monitoring [delete]
func (h *ThanosDeploymentHandler) UninstallMonitoring(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallMonitoring(c.Request.Context(), stackUUID)
	if err != nil {
		logger.Error("failed to uninstall monitoring", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Status
// @Description	Backup Status
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/status [get]
func (h *ThanosDeploymentHandler) BackupStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupStatus(c.Request.Context(), stackUUID)
	if err != nil {
		logger.Error("failed to get backup status", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Checkpoints
// @Description	Backup Checkpoints
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/checkpoints [get]
func (h *ThanosDeploymentHandler) BackupCheckpoints(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.BackupRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupCheckpoints(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to get backup checkpoints", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Snapshot
// @Description	Backup Snapshot
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/snapshot [post]
func (h *ThanosDeploymentHandler) BackupSnapshot(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupSnapshot(c.Request.Context(), stackUUID)
	if err != nil {
		logger.Error("failed to backup snapshot", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Restore
// @Description	Backup Restore
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/restore [post]
func (h *ThanosDeploymentHandler) BackupRestore(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.BackupRestoreRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupRestore(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to backup restore", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Configure
// @Description	Backup Configure
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/configure [post]
func (h *ThanosDeploymentHandler) BackupConfigure(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.BackupConfigureRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupConfigure(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to backup configure", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Attach
// @Description	Backup Attach
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/attach [post]
func (h *ThanosDeploymentHandler) BackupAttach(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.BackupAttachRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupAttach(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to backup attach", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Backup Cleanup
// @Description	Backup Cleanup
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/backup/cleanup [delete]
func (h *ThanosDeploymentHandler) BackupCleanup(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.BackupCleanup(c.Request.Context(), stackUUID)
	if err != nil {
		logger.Error("failed to backup cleanup", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Disable Email Alert
// @Description	Disable Email Alert Configuration
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/monitoring/disable-email [delete]
func (h *ThanosDeploymentHandler) DisableEmailAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.DisableEmailAlert(c.Request.Context(), stackUUID)
	if err != nil {
		logger.Error("failed to disable email alert", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Disable Telegram Alert
// @Description	Disable Telegram Alert Configuration
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/monitoring/disable-telegram [delete]
func (h *ThanosDeploymentHandler) DisableTelegramAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.DisableTelegramAlert(c.Request.Context(), stackUUID)
	if err != nil {
		logger.Error("failed to disable telegram alert", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Update Email Alert Configuration
// @Description	Update Email Alert Configuration
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id		path		string							true	"Thanos Stack ID"
// @Param			request	body		dtos.UpdateEmailConfigRequest	true	"Update Email Config Request"
// @Success		200		{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/monitoring/update-email [put]
func (h *ThanosDeploymentHandler) UpdateEmailAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.UpdateEmailConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UpdateEmailAlert(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to update email alert", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Update Telegram Alert Configuration
// @Description	Update Telegram Alert Configuration
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id		path		string								true	"Thanos Stack ID"
// @Param			request	body		dtos.UpdateTelegramConfigRequest	true	"Update Telegram Config Request"
// @Success		200		{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/monitoring/update-telegram [put]
func (h *ThanosDeploymentHandler) UpdateTelegramAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.UpdateTelegramConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UpdateTelegramAlert(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to update telegram alert", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Install Cross Chain Bridge
// @Description	Install Cross Chain Bridge
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id		path		string								true	"Thanos Stack ID"
// @Param			request	body		dtos.InstallCrossChainBridgeRequest	true	"Install Cross Chain Bridge Request"
// @Success		200		{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/cross-trade [post]
func (h *ThanosDeploymentHandler) InstallCrossChainBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.InstallCrossChainBridgeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}
	response, err := h.ThanosDeploymentService.InstallCrossChainBridge(c.Request.Context(), stackUUID, request)
	if err != nil {
		logger.Error("failed to install cross chain bridge", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Uninstall Cross Chain Bridge
// @Description	Uninstall Cross Chain Bridge
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/cross-trade/{integrationID} [delete]
func (h *ThanosDeploymentHandler) UninstallCrossChainBridge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	mode := c.Query("mode")
	if mode == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "mode is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallCrossChainBridge(c.Request.Context(), stackUUID, mode)
	if err != nil {
		logger.Error("failed to uninstall cross chain bridge", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Install Uptime Service
// @Description	Install Uptime Service
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/system-pulse [post]
func (h *ThanosDeploymentHandler) InstallUptimeService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.InstallUptimeService(c, id)
	if err != nil {
		logger.Error("failed to install uptime service", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Uninstall Uptime Service
// @Description	Uninstall Uptime Service
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"Thanos Stack ID"
// @Success		200	{object}	entities.Response
// @Router			/stacks/thanos/{id}/integrations/system-pulse [delete]
func (h *ThanosDeploymentHandler) UninstallUptimeService(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.UninstallUptimeService(c, id)
	if err != nil {
		logger.Error("failed to uninstall uptime service", zap.Error(err), zap.String("id", id))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) CancelIntegration(c *gin.Context) {
	stackUUID, integrationUUID, handled := parseAndValidateIntegrationIDs(c)
	if handled {
		return
	}

	response, err := h.ThanosDeploymentService.CancelIntegration(c.Request.Context(), stackUUID, integrationUUID)
	if err != nil {
		logger.Error("failed to cancel integration", zap.Error(err),
			zap.String("id", stackUUID.String()),
			zap.String("integrationId", integrationUUID.String()))
	}
	c.JSON(int(response.Status), response)
}

func (h *ThanosDeploymentHandler) RetryIntegration(c *gin.Context) {
	stackUUID, integrationUUID, handled := parseAndValidateIntegrationIDs(c)
	if handled {
		return
	}

	response, err := h.ThanosDeploymentService.RetryIntegration(c.Request.Context(), stackUUID, integrationUUID)
	if err != nil {
		logger.Error("failed to retry integration", zap.Error(err),
			zap.String("id", stackUUID.String()),
			zap.String("integrationId", integrationUUID.String()))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Deploy New L2 Chain
// @Description	Deploy New L2 Chain for Cross Trade
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			id		path		string						true	"Thanos Stack ID"
// @Param			request	body		dtos.DeployNewL2ChainRequest	true	"Deploy New L2 Chain Request"
// @Success		200		{object}	entities.Response
// @Failure		400		{object}	entities.Response
// @Failure		401		{object}	map[string]interface{}
// @Failure		404		{object}	entities.Response
// @Router			/stacks/thanos/{id}/cross-trade/deploy-l2-chain [post]
func (h *ThanosDeploymentHandler) DeployNewL2Chain(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}
	var request dtos.DeployNewL2ChainRequest
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

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.DeployNewL2Chain(
		c.Request.Context(),
		stackUUID,
		string(request.Mode),
		request,
	)
	if err != nil {
		logger.Error("failed to deploy new L2 chain", zap.Error(err), zap.String("id", id), zap.String("mode", string(request.Mode)))
	}
	c.JSON(int(response.Status), response)
}

// @Summary		Register Tokens
// @Description	Register Tokens for Cross Trade
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			id		path		string						true	"Thanos Stack ID"
// @Param			request	body		dtos.RegisterTokensAPIRequest	true	"Register Tokens Request"
// @Success		200		{object}	entities.Response
// @Failure		400		{object}	entities.Response
// @Failure		401		{object}	map[string]interface{}
// @Failure		404		{object}	entities.Response
// @Router			/stacks/thanos/{id}/cross-trade/register-tokens [post]
func (h *ThanosDeploymentHandler) RegisterTokens(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return
	}

	var request dtos.RegisterTokensAPIRequest
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

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return
	}

	response, err := h.ThanosDeploymentService.RegisterTokens(
		c.Request.Context(),
		stackUUID,
		string(request.Mode),
		request,
	)
	if err != nil {
		logger.Error("failed to register tokens", zap.Error(err), zap.String("id", id), zap.String("mode", string(request.Mode)))
	}
	c.JSON(int(response.Status), response)
}

// parseAndValidateIntegrationIDs extracts and validates stack and integration IDs from request params
func parseAndValidateIntegrationIDs(c *gin.Context) (stackUUID uuid.UUID, integrationUUID uuid.UUID, handled bool) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "id is required",
			Data:    nil,
		})
		return uuid.Nil, uuid.Nil, true
	}

	integrationId := c.Param("integrationId")
	if integrationId == "" {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "integrationId is required",
			Data:    nil,
		})
		return uuid.Nil, uuid.Nil, true
	}

	stackUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid stack id format",
			Data:    nil,
		})
		return uuid.Nil, uuid.Nil, true
	}

	integrationUUID, err = uuid.Parse(integrationId)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid integration id format",
			Data:    nil,
		})
		return uuid.Nil, uuid.Nil, true
	}

	return stackUUID, integrationUUID, false
}
