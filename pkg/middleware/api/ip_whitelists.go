package api

import (
	"fmt"
	"net/http"

	"git.devucc.name/dependencies/utilities/commons/middleware"
	"github.com/gin-gonic/gin"
)

func IPWhitelist() gin.HandlerFunc {
	return func(c *gin.Context) {

		clientIp := c.ClientIP()
		if gin.Mode() == gin.TestMode {
			clientIp = c.Request.Header.Get("X-Real-IP")
		}
		fmt.Println(clientIp)

		if ok := middleware.IPWhitelist(clientIp); ok {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
