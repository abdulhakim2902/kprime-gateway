package api

import (
	"fmt"
	"net/http"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/middleware"
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
