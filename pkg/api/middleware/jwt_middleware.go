package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/services"
)

type JWTMiddleware struct {
	jwtService *services.JWTService
}

func NewJWTMiddleware(jwtService *services.JWTService) *JWTMiddleware {
	return &JWTMiddleware{
		jwtService: jwtService,
	}
}

// AuthMiddleware validates JWT token and sets user information in context
// If roles are provided, it also checks if the user has one of the required roles
func (m *JWTMiddleware) AuthMiddleware(requiredRoles ...entities.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// Check if the header starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		// Extract token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate token
		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID.String())
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		// Check role requirements if any are specified
		if len(requiredRoles) > 0 {
			hasRequiredRole := false
			for _, requiredRole := range requiredRoles {
				if claims.Role == requiredRole {
					hasRequiredRole = true
					break
				}
			}

			if !hasRequiredRole {
				rolesStr := make([]string, len(requiredRoles))
				for i, role := range requiredRoles {
					rolesStr[i] = string(role)
				}
				c.JSON(http.StatusForbidden, gin.H{
					"error":          "insufficient permissions",
					"required_roles": rolesStr,
					"user_role":      string(claims.Role),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// Convenience methods for common use cases
func (m *JWTMiddleware) RequireAuth() gin.HandlerFunc {
	return m.AuthMiddleware()
}

func (m *JWTMiddleware) RequireAdmin() gin.HandlerFunc {
	return m.AuthMiddleware(entities.UserRoleAdmin)
}

func (m *JWTMiddleware) RequireUser() gin.HandlerFunc {
	return m.AuthMiddleware(entities.UserRoleUser)
}

func (m *JWTMiddleware) RequireAnyRole() gin.HandlerFunc {
	return m.AuthMiddleware(entities.UserRoleAdmin, entities.UserRoleUser)
}
