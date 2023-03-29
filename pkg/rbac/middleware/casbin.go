package middleware

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

func Authorize(obj string, act string, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("authorization")
		tokenString := authorization[7:]
		token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			token, _ := t.Method.(*jwt.SigningMethodHMAC)
            
            return token, nil
		})
		claims := token.Claims.(jwt.MapClaims);
		c.Set("role", claims["role"])
		c.Set("userID", claims["userID"])

		sub, existed := c.Get("role")
		if !existed {
			c.AbortWithStatusJSON(401, gin.H{"msg": "User hasn't logged in yet"})
			return
		}

		// Load policy from Database
		err := enforcer.LoadPolicy()
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"msg": "Failed to load policy from DB"})
			return
		}

		// Casbin enforces policy
		ok, err := enforcer.Enforce(fmt.Sprint(sub), obj, act)

		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"msg": "Error occurred when authorizing user"})
			return
		}

		if !ok {
			c.AbortWithStatusJSON(403, gin.H{"msg": "You are not authorized for this action"})
			return
		}
		c.Next()
	}
}