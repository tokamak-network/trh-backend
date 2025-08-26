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

type AWSCredentialsHandler struct {
	service *configuration.AWSCredentialsService
}

func NewAWSCredentialsHandler(service *configuration.AWSCredentialsService) *AWSCredentialsHandler {
	return &AWSCredentialsHandler{
		service: service,
	}
}

// CreateAWSCredentials godoc
//
//	@Summary		Create AWS credentials
//	@Description	Create new AWS credentials with name, access key ID, and secret access key
//	@Tags			aws-credentials
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dtos.CreateAWSCredentialsRequest	true	"Create AWS credentials request"
//	@Success		201		{object}	entities.Response{data=dtos.AWSCredentialsResponse}
//	@Failure		400		{object}	entities.Response
//	@Failure		409		{object}	entities.Response
//	@Failure		500		{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/aws-credentials [post]
func (h *AWSCredentialsHandler) Create(c *gin.Context) {
	var req dtos.CreateAWSCredentialsRequest
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
		case dtos.ErrNameRequired, dtos.ErrAccessKeyIDRequired, dtos.ErrSecretAccessKeyRequired,
			dtos.ErrInvalidAccessKeyID, dtos.ErrInvalidSecretAccessKey:
			logger.Error("validation error", zap.Error(err))
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  uint64(http.StatusBadRequest),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrNameAlreadyExists:
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
		Message: "AWS credentials created successfully",
		Data:    response,
	})
}

// GetAWSCredentialsByID godoc
//
//	@Summary		Get AWS credentials by ID
//	@Description	Get AWS credentials by their unique ID
//	@Tags			aws-credentials
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"AWS credentials ID"
//	@Success		200	{object}	entities.Response{data=dtos.AWSCredentialsResponse}
//	@Failure		400	{object}	entities.Response
//	@Failure		404	{object}	entities.Response
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/aws-credentials/{id} [get]
func (h *AWSCredentialsHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid credentials ID",
			Data:    nil,
		})
		return
	}

	response, err := h.service.GetByID(id)
	if err != nil {
		switch err {
		case dtos.ErrAWSCredentialsNotFound:
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
		Message: "AWS credentials retrieved successfully",
		Data:    response,
	})
}

// GetAllAWSCredentials godoc
//
//	@Summary		Get all AWS credentials
//	@Description	Get all AWS credentials (excluding soft deleted ones)
//	@Tags			aws-credentials
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	entities.Response{data=dtos.AWSCredentialsListResponse}
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/aws-credentials [get]
func (h *AWSCredentialsHandler) GetAll(c *gin.Context) {
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
		Message: "AWS credentials retrieved successfully",
		Data:    response,
	})
}

// UpdateAWSCredentials godoc
//
//	@Summary		Update AWS credentials
//	@Description	Update existing AWS credentials by ID (partial update)
//	@Tags			aws-credentials
//	@Accept			json
//	@Produce		json
//	@Param			id		path	string	true	"AWS credentials ID"
//	@Param			request	body	dtos.UpdateAWSCredentialsRequest	true	"Update AWS credentials request"
//	@Success		200		{object}	entities.Response{data=dtos.AWSCredentialsResponse}
//	@Failure		400		{object}	entities.Response
//	@Failure		404		{object}	entities.Response
//	@Failure		409		{object}	entities.Response
//	@Failure		500		{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/aws-credentials/{id} [patch]
func (h *AWSCredentialsHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid credentials ID",
			Data:    nil,
		})
		return
	}

	var req dtos.UpdateAWSCredentialsRequest
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
		case dtos.ErrNameRequired, dtos.ErrAccessKeyIDRequired, dtos.ErrSecretAccessKeyRequired,
			dtos.ErrInvalidAccessKeyID, dtos.ErrInvalidSecretAccessKey, dtos.ErrNoFieldsToUpdate:
			c.JSON(http.StatusBadRequest, &entities.Response{
				Status:  uint64(http.StatusBadRequest),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrAWSCredentialsNotFound:
			c.JSON(http.StatusNotFound, &entities.Response{
				Status:  uint64(http.StatusNotFound),
				Message: err.Error(),
				Data:    nil,
			})
		case dtos.ErrNameAlreadyExists:
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
		Message: "AWS credentials updated successfully",
		Data:    response,
	})
}

// DeleteAWSCredentials godoc
//
//	@Summary		Delete AWS credentials
//	@Description	Soft delete AWS credentials by ID
//	@Tags			aws-credentials
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"AWS credentials ID"
//	@Success		200	{object}	entities.Response
//	@Failure		400	{object}	entities.Response
//	@Failure		404	{object}	entities.Response
//	@Failure		500	{object}	entities.Response
//	@Security		BearerAuth
//	@Router			/configuration/aws-credentials/{id} [delete]
func (h *AWSCredentialsHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid credentials ID",
			Data:    nil,
		})
		return
	}

	err = h.service.Delete(id)
	if err != nil {
		switch err.Error() {
		case "aws credentials not found":
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
		Message: "AWS credentials deleted successfully",
		Data:    nil,
	})
}

func (h *AWSCredentialsHandler) GetAvailableRegions(c *gin.Context) {
	var req dtos.GetAvailableRegionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, &entities.Response{
			Status:  uint64(http.StatusBadRequest),
			Message: "invalid request",
			Data:    nil,
		})
		return
	}
	response, err := h.service.GetAvailableRegions(&req)
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
		Message: "AWS regions retrieved successfully",
		Data:    response,
	})
}
