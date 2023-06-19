package middleware

import (
	"encoding/base64"
	"encoding/json"
	authSvc "gateway/internal/user/service"
	"gateway/pkg/memdb"
	"io"
	"net/http"
	"strings"

	userSchema "gateway/schema"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"github.com/gin-gonic/gin"
)

func Authenticate(memDb *memdb.Schemas) gin.HandlerFunc {
	return func(c *gin.Context) {
		isPrivateMethod := false

		switch c.Request.Method {
		case "GET":
			method := c.Param("type")

			isPrivateMethod = method == "private"
		case "POST":
			body, err := io.ReadAll(c.Request.Body)
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
		}

		if isPrivateMethod {
			authorization := strings.Split(c.GetHeader("Authorization"), " ")
			if len(authorization) != 2 {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			var userId, clientSecret string
			switch authorization[0] {
			case "Bearer":
				claim, err := authSvc.ClaimJWT(nil, authorization[1])
				if err != nil {
					logs.Log.Error().Err(err).Msg("")

					c.AbortWithError(http.StatusUnauthorized, err)
					return
				}

				userId = claim.UserID
			case "deri-hmac-sha256":
				hmac := NewHmac()
				sig, err := hmac.DecodeSignature(authorization[1], c)
				if err != nil {
					c.AbortWithError(http.StatusUnauthorized, err)
					return
				}

				users := memDb.User.Find("id")
				if users == nil {
					c.AbortWithStatus(http.StatusInternalServerError)
					return

				}

				for _, user := range users {
					if usr, ok := user.(userSchema.User); ok {
						for _, key := range usr.ClientIds {
							if strings.HasPrefix(key, sig.ClientId) {
								userId = usr.ID
								clientSecret = strings.Split(key, ":")[1]
								goto VERIFY_SIGNATURE
							}
						}
					}
				}

			VERIFY_SIGNATURE:
				ok := sig.Verify(clientSecret)
				if !ok {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
			case "Basic":
				decoded, err := base64.StdEncoding.DecodeString(authorization[1])
				if err != nil {
					c.AbortWithError(http.StatusUnauthorized, err)
					return
				}

				users := memDb.User.Find("id")
				if users == nil {
					c.AbortWithStatus(http.StatusInternalServerError)
					return

				}

				for _, user := range users {
					if usr, ok := user.(userSchema.User); ok {
						for _, key := range usr.ClientIds {
							if strings.EqualFold(key, string(decoded)) {
								userId = usr.ID
								goto SET_USERID
							}
						}
					}
				}
			default:
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

		SET_USERID:
			if userId == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Set("userID", userId)
		}

		c.Next()
	}
}
