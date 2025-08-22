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

type ApiKeyHandler struct {
	service *configuration.ApiKeyService
}

func NewApiKeyHandler(service *configuration.ApiKeyService) *ApiKeyHandler {
	return &ApiKeyHandler{
		service: service,
	}
}

// CreateApiKey godoc
//
//	@Summary		Create API Key
//	@Description	Create new API key configuration with key and type
//	@Tags			api-key
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dtos.CreateApiKeyRequest	true	"Create API Key request"
//	@Success		201		{object}	entities.Response{data=dtos.ApiKeyResponse}
//	@Failure		400		{object}	entities.Response
//	@Failure		500		{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/api-key [post]
func (h *ApiKeyHandler) Create(c *gin.Context) {
	var req dtos.CreateApiKeyRequest
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
		case dtos.ErrApiKeyRequired, dtos.ErrApiKeyTypeRequired:
			logger.Error("validation error", zap.Error(err))
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  uint64(http.StatusBadRequest),
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
		Message: "API key created successfully",
		Data:    response,
	})
}

// GetApiKeyByID godoc
//
//	@Summary		Get API Key by ID
//	@Description	Get API key configuration by its unique ID
//	@Tags			api-key
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"API Key ID"
//	@Success		200	{object}	entities.Response{data=dtos.ApiKeyResponse}
//	@Failure		400	{object}	entities.Response
//	@Failure		404	{object}	entities.Response
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/api-key/{id} [get]
func (h *ApiKeyHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid API key ID",
			Data:    nil,
		})
		return
	}

	response, err := h.service.GetByID(id)
	if err != nil {
		switch err {
		case dtos.ErrApiKeyNotFound:
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
		Message: "API key retrieved successfully",
		Data:    response,
	})
}

// GetAllApiKeys godoc
//
//	@Summary		Get all API Keys
//	@Description	Get all API key configurations (excluding soft deleted ones)
//	@Tags			api-key
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	entities.Response{data=dtos.ApiKeyListResponse}
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/api-key [get]
func (h *ApiKeyHandler) GetAll(c *gin.Context) {
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
		Message: "API keys retrieved successfully",
		Data:    response,
	})
}

// UpdateApiKey godoc
//
//	@Summary		Update API Key
//	@Description	Update existing API key configuration by ID (partial update)
//	@Tags			api-key
//	@Accept			json
//	@Produce		json
//	@Param			id		path	string	true	"API Key ID"
//	@Param			request	body	dtos.UpdateApiKeyRequest	true	"Update API Key request"
//	@Success		200		{object}	entities.Response{data=dtos.ApiKeyResponse}
//	@Failure		400		{object}	entities.Response
//	@Failure		404		{object}	entities.Response
//	@Failure		500		{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/api-key/{id} [patch]
func (h *ApiKeyHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid API key ID",
			Data:    nil,
		})
		return
	}

	var req dtos.UpdateApiKeyRequest
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
		case dtos.ErrApiKeyRequired, dtos.ErrApiKeyTypeRequired, dtos.ErrNoFieldsToUpdate:
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  uint64(http.StatusBadRequest),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrApiKeyNotFound:
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
		Message: "API key updated successfully",
		Data:    response,
	})
}

// DeleteApiKey godoc
//
//	@Summary		Delete API Key
//	@Description	Soft delete API key configuration by ID
//	@Tags			api-key
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"API Key ID"
//	@Success		200	{object}	entities.Response
//	@Failure		400	{object}	entities.Response
//	@Failure		404	{object}	entities.Response
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/api-key/{id} [delete]
func (h *ApiKeyHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid API key ID",
			Data:    nil,
		})
		return
	}

	err = h.service.Delete(id)
	if err != nil {
		switch err.Error() {
		case "api key not found":
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
		Message: "API key deleted successfully",
		Data:    nil,
	})
}
