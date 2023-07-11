package api

import (
	"net/http"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/middleware"
	"github.com/gin-gonic/gin"
)

func BasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if ok := middleware.BasicAuth(authHeader); !ok {
			c.Header("WWW-Authenticate", "Basic realm=Restricted")
			c.AbortWithStatus(http.StatusUnauthorized)
			return

		}

		c.Next()
	}
}
