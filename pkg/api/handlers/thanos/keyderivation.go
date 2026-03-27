package thanos

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

// @Summary		Derive Operator Keys
// @Description	Derive BIP44 HD wallet private keys for admin, sequencer, batcher, and proposer
//
//	from a BIP39 seed phrase. Keys are computed in-memory and never persisted.
//	When l1RpcUrl is supplied, the derived addresses are also queried for their ETH balance.
//
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		dtos.DeriveKeysRequest	true	"Derive Keys Request"
// @Success		200		{object}	entities.Response{data=dtos.DeriveKeysResponse}
// @Failure		400		{object}	entities.Response
// @Failure		500		{object}	entities.Response
// @Router			/stacks/thanos/derive-keys [post]
func (h *ThanosDeploymentHandler) DeriveKeys(c *gin.Context) {
	var req dtos.DeriveKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	// DeriveKeys encodes all domain errors into resp.Status and never returns a non-nil
	// error. The err branch below is a defensive guard for future service changes.
	resp, err := h.ThanosDeploymentService.DeriveKeys(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to derive keys", zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "internal server error",
		})
		return
	}

	userID, _ := c.Get("user_id")
	logger.Info("derive-keys invoked",
		zap.Any("userID", userID),
		zap.String("clientIP", c.ClientIP()),
	)

	c.JSON(int(resp.Status), resp)
}

// @Summary		Check Account Balances
// @Description	Fetch ETH balances on L1 for the given operator addresses.
//
//	Use this before deployment to verify that all operator accounts are funded.
//
// @Tags			Thanos Stack
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			request	body		dtos.CheckAccountBalancesRequest	true	"Check Account Balances Request"
// @Success		200		{object}	entities.Response{data=dtos.CheckAccountBalancesResponse}
// @Failure		400		{object}	entities.Response
// @Failure		500		{object}	entities.Response
// @Router			/stacks/thanos/check-account-balances [post]
func (h *ThanosDeploymentHandler) CheckAccountBalances(c *gin.Context) {
	var req dtos.CheckAccountBalancesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	resp, err := h.ThanosDeploymentService.CheckAccountBalances(c.Request.Context(), &req)
	if err != nil {
		logger.Error("failed to check account balances", zap.Error(err))
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "internal server error",
		})
		return
	}

	c.JSON(int(resp.Status), resp)
}
