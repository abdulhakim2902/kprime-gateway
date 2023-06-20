package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func BasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader != "Basic "+os.Getenv("PROTECT_BASIC") {
			c.Header("WWW-Authenticate", "Basic realm=Restricted")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
