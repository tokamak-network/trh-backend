package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login godoc
//
//	@Summary		Login user
//	@Description	Login user with email and password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dtos.LoginRequest	true	"Login request"
//	@Success		200		{object}	dtos.AuthResponse
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dtos.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.authService.Login(&req)
	if err != nil {
		switch err {
		case dtos.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case dtos.ErrInvalidEmail, dtos.ErrPasswordRequired:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetProfile godoc
//
//	@Summary		Get user profile
//	@Description	Get current user profile information
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	dtos.UserResponse
//	@Failure		401	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		switch err {
		case dtos.ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetUsers godoc
//
//	@Summary		Get all users (Admin only)
//	@Description	Get paginated list of all users
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page	query		int	false	"Page number (default: 1)"
//	@Param			limit	query		int	false	"Items per page (default: 10, max: 100)"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Failure		403		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/auth/users [get]
func (h *AuthHandler) GetUsers(c *gin.Context) {
	// This would need to be implemented in the auth service
	// For now, just return a placeholder response
	c.JSON(http.StatusOK, gin.H{"message": "Get users endpoint - to be implemented"})
}
