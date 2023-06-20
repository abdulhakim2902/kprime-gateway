package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func IPWhitelist() gin.HandlerFunc {
	return func(c *gin.Context) {
		whitelistIPs := make(map[string]bool)
		if len(os.Getenv("PROTECT_IP_WHITELISTS")) > 0 {
			ips := strings.Split(os.Getenv("PROTECT_IP_WHITELISTS"), ",")

			for _, ip := range ips {
				whitelistIPs[ip] = true
			}
		}
		if whitelistIPs[c.ClientIP()] {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
