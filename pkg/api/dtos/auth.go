package dtos

import (
	"regexp"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

type LoginRequest struct {
	Email    string `json:"email" binding:"required" validate:"email"`
	Password string `json:"password" binding:"required"`
}

func (r *LoginRequest) Validate() error {
	if !emailRegex.MatchString(r.Email) {
		return ErrInvalidEmail
	}

	if r.Password == "" {
		return ErrPasswordRequired
	}

	return nil
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID    string            `json:"id"`
	Email string            `json:"email"`
	Role  entities.UserRole `json:"role"`
}

// Custom errors
var (
	ErrInvalidEmail       = &ValidationError{Message: "invalid email format"}
	ErrPasswordRequired   = &ValidationError{Message: "password is required"}
	ErrUserNotFound       = &ValidationError{Message: "user not found"}
	ErrInvalidCredentials = &ValidationError{Message: "invalid credentials"}
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
