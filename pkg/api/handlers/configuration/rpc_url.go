package configuration

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/services/configuration"
	"go.uber.org/zap"
)

type RPCUrlHandler struct {
	service *configuration.RPCUrlService
}

func NewRPCUrlHandler(service *configuration.RPCUrlService) *RPCUrlHandler {
	return &RPCUrlHandler{
		service: service,
	}
}

// CreateRPCUrl godoc
//
//	@Summary		Create RPC URL
//	@Description	Create new RPC URL configuration with name, URL, type, and network
//	@Tags			rpc-url
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dtos.CreateRPCUrlRequest	true	"Create RPC URL request"
//	@Success		201		{object}	entities.Response{data=dtos.RPCUrlResponse}
//	@Failure		400		{object}	entities.Response
//	@Failure		409		{object}	entities.Response
//	@Failure		500		{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/rpc-url [post]
func (h *RPCUrlHandler) Create(c *gin.Context) {
	var req dtos.CreateRPCUrlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.service.Create(&req)
	if err != nil {
		switch err {
		case dtos.ErrNameRequired, dtos.ErrRpcUrlRequired, dtos.ErrInvalidRpcUrlFormat,
			dtos.ErrInvalidRpcType, dtos.ErrInvalidNetworkType:
			logger.Error("validation error", zap.Error(err))
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  uint64(http.StatusBadRequest),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrRpcUrlNameExists:
			logger.Error("name already exists", zap.Error(err))
			c.JSON(http.StatusConflict, &entities.Response{
				Status:  uint64(http.StatusConflict),
				Message: err.Error(),
				Data:    nil,
			})
		default:
			logger.Error("internal server error", zap.Error(err))
			c.JSON(http.StatusInternalServerError, &entities.Response{
				Status:  uint64(http.StatusInternalServerError),
				Message: "internal server error",
				Data:    nil,
			})
		}
		return
	}

	c.JSON(http.StatusCreated, &entities.Response{
		Status:  uint64(http.StatusCreated),
		Message: "RPC URL created successfully",
		Data:    response,
	})
}

// GetRPCUrlByID godoc
//
//	@Summary		Get RPC URL by ID
//	@Description	Get RPC URL configuration by its unique ID
//	@Tags			rpc-url
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"RPC URL ID"
//	@Success		200	{object}	entities.Response{data=dtos.RPCUrlResponse}
//	@Failure		400	{object}	entities.Response
//	@Failure		404	{object}	entities.Response
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/rpc-url/{id} [get]
func (h *RPCUrlHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid RPC URL ID",
			Data:    nil,
		})
		return
	}

	response, err := h.service.GetByID(id)
	if err != nil {
		switch err {
		case dtos.ErrRpcUrlNotFound:
			c.JSON(http.StatusNotFound, &entities.Response{
				Status:  uint64(http.StatusNotFound),
				Message: err.Error(),
				Data:    nil,
			})
		default:
			c.JSON(http.StatusInternalServerError, &entities.Response{
				Status:  uint64(http.StatusInternalServerError),
				Message: "internal server error",
				Data:    nil,
			})
		}
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  uint64(http.StatusOK),
		Message: "RPC URL retrieved successfully",
		Data:    response,
	})
}

// GetAllRPCUrls godoc
//
//	@Summary		Get all RPC URLs
//	@Description	Get all RPC URL configurations (excluding soft deleted ones)
//	@Tags			rpc-url
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	entities.Response{data=dtos.RPCUrlListResponse}
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/rpc-url [get]
func (h *RPCUrlHandler) GetAll(c *gin.Context) {
	response, err := h.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, &entities.Response{
			Status:  uint64(http.StatusInternalServerError),
			Message: "internal server error",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  uint64(http.StatusOK),
		Message: "RPC URLs retrieved successfully",
		Data:    response,
	})
}

// UpdateRPCUrl godoc
//
//	@Summary		Update RPC URL
//	@Description	Update existing RPC URL configuration by ID (partial update)
//	@Tags			rpc-url
//	@Accept			json
//	@Produce		json
//	@Param			id		path	string	true	"RPC URL ID"
//	@Param			request	body	dtos.UpdateRPCUrlRequest	true	"Update RPC URL request"
//	@Success		200		{object}	entities.Response{data=dtos.RPCUrlResponse}
//	@Failure		400		{object}	entities.Response
//	@Failure		404		{object}	entities.Response
//	@Failure		409		{object}	entities.Response
//	@Failure		500		{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/rpc-url/{id} [patch]
func (h *RPCUrlHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid RPC URL ID",
			Data:    nil,
		})
		return
	}

	var req dtos.UpdateRPCUrlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	response, err := h.service.Update(id, &req)
	if err != nil {
		switch err {
		case dtos.ErrNameRequired, dtos.ErrRpcUrlRequired, dtos.ErrInvalidRpcUrlFormat,
			dtos.ErrInvalidRpcType, dtos.ErrInvalidNetworkType, dtos.ErrNoFieldsToUpdate:
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  uint64(http.StatusBadRequest),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrRpcUrlNotFound:
			c.JSON(http.StatusNotFound, &entities.Response{
				Status:  uint64(http.StatusNotFound),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrRpcUrlNameExists:
			c.JSON(http.StatusConflict, &entities.Response{
				Status:  uint64(http.StatusConflict),
				Message: err.Error(),
				Data:    nil,
			})
		default:
			c.JSON(http.StatusInternalServerError, &entities.Response{
				Status:  uint64(http.StatusInternalServerError),
				Message: "internal server error",
				Data:    nil,
			})
		}
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  uint64(http.StatusOK),
		Message: "RPC URL updated successfully",
		Data:    response,
	})
}

// DeleteRPCUrl godoc
//
//	@Summary		Delete RPC URL
//	@Description	Soft delete RPC URL configuration by ID
//	@Tags			rpc-url
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"RPC URL ID"
//	@Success		200	{object}	entities.Response
//	@Failure		400	{object}	entities.Response
//	@Failure		404	{object}	entities.Response
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/rpc-url/{id} [delete]
func (h *RPCUrlHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid RPC URL ID",
			Data:    nil,
		})
		return
	}

	err = h.service.Delete(id)
	if err != nil {
		switch err.Error() {
		case "rpc url not found":
			c.JSON(http.StatusNotFound, &entities.Response{
				Status:  uint64(http.StatusNotFound),
				Message: err.Error(),
				Data:    nil,
			})
		default:
			c.JSON(http.StatusInternalServerError, &entities.Response{
				Status:  uint64(http.StatusInternalServerError),
				Message: "internal server error",
				Data:    nil,
			})
		}
		return
	}

	c.JSON(http.StatusOK, &entities.Response{
		Status:  uint64(http.StatusOK),
		Message: "RPC URL deleted successfully",
		Data:    nil,
	})
}
