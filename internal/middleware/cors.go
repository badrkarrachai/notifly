package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a configured CORS middleware.
func CORS(origins, methods, headers []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: origins,
		AllowMethods: methods,
		AllowHeaders: headers,
	})
}
