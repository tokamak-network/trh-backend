package services

import (
	"os"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/schemas"
)

type AuthService struct {
	userRepo   *repositories.UserRepository
	jwtService *JWTService
}

func NewAuthService(userRepo *repositories.UserRepository, jwtService *JWTService) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtService: jwtService,
	}
}

// CreateDefaultAdmin creates a default admin account if no users exist
func (s *AuthService) CreateDefaultAdmin() error {
	// Check if any users exist
	users, err := s.userRepo.List(0, 1)
	if err != nil {
		return err
	}

	// If users exist, don't create default admin
	if len(users) > 0 {
		return nil
	}

	// Get default admin credentials from environment variables
	defaultAdminEmail := os.Getenv("DEFAULT_ADMIN_EMAIL")
	defaultAdminPassword := os.Getenv("DEFAULT_ADMIN_PASSWORD")

	// Use fallback values if environment variables are not set
	if defaultAdminEmail == "" {
		defaultAdminEmail = "admin@gmail.com"
	}
	if defaultAdminPassword == "" {
		defaultAdminPassword = "admin"
	}

	// Create default admin user
	user := &schemas.User{
		Email:    defaultAdminEmail,
		Password: defaultAdminPassword,
		Role:     entities.UserRoleAdmin,
	}

	// Hash password
	if err := user.HashPassword(); err != nil {
		return err
	}

	// Save user to database
	return s.userRepo.Create(user)
}

func (s *AuthService) Login(req *dtos.LoginRequest) (*dtos.AuthResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Find user by email
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, dtos.ErrInvalidCredentials
	}

	// Check password
	if !user.CheckPassword(req.Password) {
		return nil, dtos.ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &dtos.AuthResponse{
		Token: token,
		User: dtos.UserResponse{
			ID:    user.ID.String(),
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}

func (s *AuthService) GetUserByID(userID uuid.UUID) (*dtos.UserResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, dtos.ErrUserNotFound
	}

	return &dtos.UserResponse{
		ID:    user.ID.String(),
		Email: user.Email,
		Role:  user.Role,
	}, nil
}
