package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"
)

// RequestLoggerMiddleware logs each HTTP request with details about the request and response
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		// Log request details
		if query != "" {
			path = fmt.Sprintf("%s?%s", path, query)
		}

		// Prepare fields for logging
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Int("size", bodySize),
			zap.Duration("latency", latency),
			zap.String("ip", clientIP),
			zap.String("user-agent", userAgent),
		}

		// Add error if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
		}

		// Use different log levels based on status code
		switch {
		case statusCode >= 500:
			logger.Error("HTTP Request", fields...)
		case statusCode >= 400:
			logger.Warn("HTTP Request", fields...)
		default:
			logger.Info("HTTP Request", fields...)
		}
	}
}
