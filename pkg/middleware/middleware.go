package middleware

import (
	authSvc "gateway/internal/user/service"
	"net/http"
	"strings"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/gin-gonic/gin"
)

func Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.Split(c.GetHeader("Authorization"), " ")
		if len(token) != 2 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claim, err := authSvc.ClaimJWT(nil, token[1])
		if err != nil {
			logs.Log.Error().Err(err).Msg("")

			c.AbortWithError(http.StatusUnauthorized, err)
			return
		}

		c.Set("userID", claim.UserID)

		c.Next()
	}
}
