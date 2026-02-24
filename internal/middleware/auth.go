package middleware

import (
	"crypto/subtle"
	"net/http"

	"notifly/internal/common"

	"github.com/gin-gonic/gin"
)

// Auth returns middleware that validates the X-API-Key header against configured keys.
// This is service-to-service authentication — not JWT-based.
func Auth(validKeys []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			common.Error(c, http.StatusUnauthorized, "missing X-API-Key header")
			c.Abort()
			return
		}

		if !isValidKey(apiKey, validKeys) {
			common.Error(c, http.StatusUnauthorized, "invalid API key")
			c.Abort()
			return
		}

		c.Next()
	}
}

// isValidKey checks the provided key against the list of valid keys using constant-time comparison.
func isValidKey(key string, validKeys []string) bool {
	for _, valid := range validKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(valid)) == 1 {
			return true
		}
	}
	return false
}
