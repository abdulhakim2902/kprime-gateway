package middleware

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	authSvc "gateway/internal/user/service"
	"gateway/pkg/hmac"
	"gateway/pkg/memdb"
	"gateway/pkg/protocol"
	"io"
	"net/http"
	"strings"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/gin-gonic/gin"
)

func Authenticate() gin.HandlerFunc {
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
				logs.Log.Err(err).Msg("")
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

			var userId, userRole string
			switch authorization[0] {
			case "Bearer":
				claim, err := authSvc.ClaimJWT(nil, authorization[1])
				if err != nil {
					logs.Log.Error().Err(err).Msg("")
					errMsg := protocol.ErrorMessage{
						Message:        err.Error(),
						Data:           protocol.ReasonMessage{},
						HttpStatusCode: http.StatusBadRequest,
					}
					m := protocol.RPCResponseMessage{
						JSONRPC: "2.0",
						Error:   &errMsg,
						Testnet: true,
					}
					c.AbortWithStatusJSON(http.StatusUnauthorized, m)
					return
				}

				userId = claim.UserID
				userRole = claim.UserRole
			case "deri-hmac-sha256":
				hmac := hmac.New()
				sig, err := hmac.DecodeSignature(authorization[1], c)
				if err != nil {
					logs.Log.Error().Err(err).Msg("")

					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}

				user, credential, reason := memdb.MDBFindUserAndCredentialWithKey(sig.ClientId)
				if reason != nil {
					_, status, msg := reason.Code()

					c.AbortWithError(status, errors.New(msg))
					return
				}

				ok := sig.Verify(credential.Secret)
				if !ok {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}

				userId = user.ID
				userRole = user.Role.String()
			case "Basic":
				decoded, err := base64.StdEncoding.DecodeString(authorization[1])
				if err != nil {
					c.AbortWithError(http.StatusUnauthorized, err)
					return
				}

				user, _, reason := memdb.MDBFindUserAndCredentialWithKey(strings.Split(string(decoded), ":")[0])
				if reason != nil {
					_, status, msg := reason.Code()

					c.AbortWithError(status, errors.New(msg))
					return
				}

				userId = user.ID
				userRole = user.Role.String()
			default:
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			if userId == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Set("userID", userId)
			c.Set("userRole", userRole)
		}

		c.Next()
	}
}
