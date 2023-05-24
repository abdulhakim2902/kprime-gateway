package middleware

import (
	"gateway/internal/deribit/model"
	authSvc "gateway/internal/user/service"
	"gateway/pkg/utils"
	"net/http"
	"strings"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/gin-gonic/gin"
)

func Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		isPrivateMethod := false

		switch c.Request.Method {
		case "GET":
			method := c.Param("type")

			isPrivateMethod = method == "private"
			break
		case "POST":
			var dto model.RequestDto[model.EmptyParams]
			if err := utils.UnmarshalAndValidate(c, &dto); err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			isPrivateMethod = strings.Contains(dto.Method, "private")
			break
		}

		if isPrivateMethod {
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
		}

		c.Next()
	}
}
