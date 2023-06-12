package middleware

import (
	"encoding/json"
	authSvc "gateway/internal/user/service"
	"io/ioutil"
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
			body, err := ioutil.ReadAll(c.Request.Body)
			if err != nil {
				logs.Log.Err(err).Msg("")
				c.AbortWithStatus(http.StatusBadRequest)
			}

			var data gin.H
			if err := json.Unmarshal(body, &data); err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			val, ok := data["method"]
			if !ok {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			_, ok = data["id"]
			if !ok {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}

			isPrivateMethod = strings.Contains(val.(string), "private")
			c.Set("body", body)
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
